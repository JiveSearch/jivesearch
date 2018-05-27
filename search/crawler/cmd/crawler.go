// Command crawler demonstrates how to run the crawler
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/abursavich/nett"
	"github.com/garyburd/redigo/redis"
	"github.com/jivesearch/jivesearch/config"
	"github.com/jivesearch/jivesearch/log"
	"github.com/jivesearch/jivesearch/search/crawler"
	"github.com/jivesearch/jivesearch/search/crawler/queue"
	"github.com/jivesearch/jivesearch/search/crawler/robots"
	"github.com/jivesearch/jivesearch/search/document"
	img "github.com/jivesearch/jivesearch/search/image"
	"github.com/olivere/elastic"
	"github.com/spf13/viper"
)

func afterFn(executionID int64, requests []elastic.BulkableRequest, resp *elastic.BulkResponse, err error) {
	// NOTE: err can be nil even if documents fail to update
	if resp != nil {
		failed := resp.Failed()
		for _, d := range failed {
			log.Info.Printf("document failed: %+v\n", d)
			log.Info.Printf(" reason: %+v\n", d.Error)
		}
	}

	if err != nil {
		panic(err)
	}
}

var (
	c        *crawler.Crawler
	duration time.Duration
)

func setup(v *viper.Viper) {
	v.SetEnvPrefix("jivesearch")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.SetDefaults(v)

	if v.GetBool("debug") {
		log.Debug.SetOutput(os.Stdout)
	}

	duration = v.GetDuration("crawler.time")

	c = crawler.New(v)

	c.HTTPClient = &http.Client{
		Transport: &http.Transport{
			Dial: (&nett.Dialer{
				Resolver: &nett.CacheResolver{TTL: 10 * time.Minute},
				IPFilter: nett.DualStack,
			}).Dial,
			DisableKeepAlives: true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// For robots.txt follow the default redirect policy of max 10 redirects
			// For all others don't follow redirects
			if strings.ToLower(req.URL.Path) == crawler.RobotsPath.Path {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				return nil
			}
			return http.ErrUseLastResponse
		},
		Timeout: v.GetDuration("crawler.timeout"),
	}
}

func main() {
	v := viper.New()
	setup(v)

	// setup Elasticsearch
	// Note: for remote URLs I can't seem to get it to work with sniffing on
	// see https://github.com/olivere/elastic/issues/312
	ri := v.GetString("elasticsearch.robots.index")
	client, err := elastic.NewClient(elastic.SetURL(v.GetString("elasticsearch.url")), elastic.SetSniff(false))
	if err != nil {
		panic(err)
	}

	bulk, err := client.BulkProcessor().
		After(afterFn).
		//BulkActions().
		Do(context.Background())

	if err != nil {
		panic(err)
	}

	defer bulk.Close()

	// setup our search index
	c.Backend = &crawler.ElasticSearch{
		ElasticSearch: &document.ElasticSearch{
			Client: client,
			Index:  v.GetString("elasticsearch.search.index"),
			Type:   v.GetString("elasticsearch.search.type"),
		},
		Bulk: bulk,
	}

	if err := c.Backend.Setup(); err != nil {
		panic(err)
	}

	// setup our image index
	c.ImageBackend = &img.ElasticSearch{
		Client: client,
		Index:  v.GetString("elasticsearch.image.index"),
		Type:   v.GetString("elasticsearch.image.type"),
		Bulk:   bulk,
	}

	if err := c.ImageBackend.Setup(); err != nil {
		panic(err)
	}

	// Setup our robots.txt cache
	c.Robots = &robots.ElasticSearch{
		Client: client,
		Bulk:   bulk,
		Index:  ri,
		Type:   v.GetString("elasticsearch.robots.type"),
	}

	exists, err := c.Robots.IndexExists()
	if err != nil {
		panic(err)
	}

	if !exists {
		if err := c.Robots.Setup(); err != nil {
			panic(err)
		}
	}

	// Setup our queue
	rds := &queue.Redis{
		RedisPool: &redis.Pool{
			MaxIdle:     v.GetInt("crawler.workers"),
			MaxActive:   v.GetInt("crawler.workers"),
			IdleTimeout: 10 * time.Second,
			Wait:        true,
			Dial: func() (redis.Conn, error) {
				cl, err := redis.Dial("tcp", fmt.Sprintf("%v:%v", v.GetString("redis.host"), v.GetString("redis.port")))
				if err != nil {
					return nil, err
				}
				return cl, err
			},
		},
	}

	defer rds.RedisPool.Close()
	c.Queue = rds

	defer c.Close()

	if err := c.Start(duration); err != nil {
		log.Info.Fatalf("%+v", err)
	}
}
