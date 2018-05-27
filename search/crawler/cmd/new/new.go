package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/queue"
	"github.com/gocolly/redisstorage"
	"github.com/jivesearch/jivesearch/config"
	"github.com/jivesearch/jivesearch/log"
	"github.com/jivesearch/jivesearch/search/crawler"
	"github.com/jivesearch/jivesearch/search/document"
	img "github.com/jivesearch/jivesearch/search/image"
	"github.com/olivere/elastic"
	"github.com/pkg/errors"
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

var now = func() time.Time { return time.Now().UTC() }

func setup(v *viper.Viper) {
	v.SetEnvPrefix("jivesearch")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.SetDefaults(v)

	if v.GetBool("debug") {
		log.Debug.SetOutput(os.Stdout)
	}
}

func main() {
	v := viper.New()
	setup(v)

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
	backend := &crawler.ElasticSearch{
		ElasticSearch: &document.ElasticSearch{
			Client: client,
			Index:  v.GetString("elasticsearch.search.index"),
			Type:   v.GetString("elasticsearch.search.type"),
		},
		Bulk: bulk,
	}

	if err := backend.Setup(); err != nil {
		panic(err)
	}

	// setup our image index
	imgBackend := &img.ElasticSearch{
		Client: client,
		Index:  v.GetString("elasticsearch.image.index"),
		Type:   v.GetString("elasticsearch.image.type"),
		Bulk:   bulk,
	}

	if err := imgBackend.Setup(); err != nil {
		panic(err)
	}

	c := colly.NewCollector(
		//colly.Async(true), // not necessary with Queue
		colly.UserAgent(v.GetString("crawler.useragent.full")),
		colly.MaxBodySize(v.GetInt("crawler.max.bytes")),
	)

	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: v.GetInt("crawler.workers"), // Parallelism of 1 will give us only 1 worker for ALL domains...
		Delay:       1 * time.Second,
	},
	)

	storage := &redisstorage.Storage{
		Address: fmt.Sprintf("%v:%v", v.GetString("redis.host"), v.GetString("redis.port")),
		DB:      0,
		Prefix:  "jivesearch",
	}

	// add storage to the collector
	err = c.SetStorage(storage)
	if err != nil {
		panic(err)
	}

	defer storage.Client.Close()

	q, err := queue.New(v.GetInt("crawler.workers"), storage)
	if err != nil {
		panic(err)
	}

	stats := &crawler.Stats{Start: now(), StatusCodes: make(map[int]int64)}

	c.OnRequest(func(r *colly.Request) {})
	c.OnError(func(r *colly.Response, err error) {
		stats.Update(r.StatusCode)
		log.Info.Println("Request URL:", r.Request.URL, "status:", r.StatusCode, "\nError:", err)
	})

	links := make(chan string)
	images := make(chan *img.Image)
	defer close(links)
	defer close(images)

	uaShort := v.GetString("crawler.useragent.short")

	errs := make(chan error)

	c.OnResponse(func(r *colly.Response) {
		stats.Update(r.StatusCode)

		lnk := r.Request.URL.String()
		log.Debug.Println(fmt.Sprintf("%d %v", r.StatusCode, lnk))

		doc, err := document.New(lnk)
		if err != nil {
			log.Debug.Println(errors.Wrapf(err, "link: %q", lnk))
			return
		}

		doc.SetStatusCode(r.StatusCode).SetCrawled(now())

		if doc.StatusCode == http.StatusOK {
			var b io.Reader = bytes.NewReader(r.Body)
			maxBytes := int64(v.GetInt("crawler.max.bytes"))
			if maxBytes > -1 {
				b = io.LimitReader(b, maxBytes)
			}
			err = doc.SetHeader(*r.Headers).
				SetPolicyFromHeader(uaShort).
				SetTokenizer(b)

			if err != nil {
				log.Debug.Printf("document parsing error: %v\n%v", doc.ID, err)
				return
			}

			// TODO: extract some of the text of pdf files.
			// Note: some html is mismarked as text/xml.
			if doc.MIME != "text/plain" && doc.MIME != "text/html" && doc.MIME != "text/xml" {
				return
			}

			queueCnt, err := q.Size()
			if err != nil {
				errs <- errors.Wrapf(err, "unable to get queue size")
				return
			}

			maxLinks := v.GetInt("crawler.max.links")
			if queueCnt > v.GetInt("crawler.max.queue.links") {
				maxLinks = 0
			}

			if err := doc.SetContent(uaShort, maxLinks, links, images,
				v.GetInt("crawler.truncate.title"), v.GetInt("crawler.truncate.keywords"), v.GetInt("crawler.truncate.description")); err != nil {
				log.Debug.Printf("document parsing error: %v\n%v", doc.ID, err)
			}

			// don't index content if not wanted or if not canonical
			if doc.SetCanonical(links); !doc.Canonical || !doc.Index {
				doc = &document.Document{
					ID:      doc.ID,
					Crawled: doc.Crawled,
					Content: document.Content{
						StatusCode: doc.StatusCode,
						Language:   doc.Language,
					},
				}
			}
		}

		if err := backend.Upsert(doc); err != nil {
			errs <- errors.Wrapf(err, "unable to insert doc: %v", doc.ID)
			return
		}
	})

	for _, lnk := range v.GetStringSlice("crawler.seeds") {
		q.AddURL(lnk)
	}

	go linkHandler(q, links, errs)
	go imageHandler(imgBackend, q, images, errs)

	go func() {
		ctx, cancel := context.WithTimeout(context.TODO(), v.GetDuration("crawler.time"))
		defer cancel()

		select {
		case <-ctx.Done():
		case err = <-errs:
			log.Info.Fatalln(err)
		}

		log.Info.Printf("elapsed: %v", stats.Elapsed().String())
		os.Exit(1)
	}()

	q.Run(c)
}

func linkHandler(q *queue.Queue, links chan string, errs chan error) {
	for lnk := range links {
		if err := q.AddURL(lnk); err != nil {
			errs <- errors.Wrapf(err, "%q", lnk)
			return
		}
	}
}

func imageHandler(b *img.ElasticSearch, q *queue.Queue, images chan *img.Image, errs chan error) {
	for img := range images {
		if err := b.Upsert(img); err != nil {
			errs <- errors.Wrapf(err, "unable to insert image: %v", img.ID)
			return
		}
	}
}
