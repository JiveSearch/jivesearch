package main

import (
	"context"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jivesearch/jivesearch/config"
	"github.com/jivesearch/jivesearch/log"
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

	e := &img.ElasticSearch{
		Client: client,
		Index:  v.GetString("elasticsearch.image.index"),
		Type:   v.GetString("elasticsearch.image.type"),
		Bulk:   bulk,
	}

	images, err := e.Uncrawled(10000)
	if err != nil {
		panic(err)
	}

	for i, image := range images {
		log.Info.Println(i, image.ID)
		// At some point we should rewrite the Flask server to Go and put it here instead...
		cl := &http.Client{
			Timeout: 2 * time.Second,
		}

		u := "http://localhost:5000/?image=" + image.ID
		resp, err := cl.Get(u)
		if err != nil {
			log.Debug.Println(err)
			continue
		}

		defer resp.Body.Close()

		bdy, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Debug.Println(err)
			continue
		}

		image.Crawled = time.Now().Format("20060102")
		// some will return a 404 status, meaning the file might be svg and unable to convert to an image format
		if resp.StatusCode == 200 {
			image.NSFW, err = strconv.ParseFloat(string(bdy), 64)
			if err != nil {
				log.Debug.Println(err)
			}

		}

		if err = e.Upsert(image); err != nil {
			panic(err)
		}
	}
}
