package image

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/olivere/elastic"
)

func TestUpsert(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		resp   string
		img    *Image
		err    error
	}{
		{
			name:   "basic",
			status: http.StatusCreated,
			resp: `{
			  "took": 27,
			  "errors": false,
			  "items": [
					{
			      "create": {
			        "_index": "images",
			        "_type": "images",
			        "_id": "AVhRlxyshqP4iSOLLnUz",
			        "_version": 1,
			        "_shards": {
			          "total": 2,
			          "successful": 1,
			          "failed": 0
			        },
			        "status": 201
			      }
				  }
				]
			}`,
			img: &Image{
				ID:      "http://www.example.com/path/to/nowhere",
				Alt:     "some image we have here",
				NSFW:    .0002,
				Crawled: "20180520",
			},
			err: nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			handler := http.NotFound
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler(w, r)
			}))
			defer ts.Close()

			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.resp))
			}

			e, err := MockService(ts.URL)
			if err != nil {
				t.Fatal(err)
			}

			e.Upsert(c.img)

			if err := e.Bulk.Flush(); err != nil {
				t.Fatal(err)
			}

			stats := e.Bulk.Stats()
			if stats.Succeeded != 1 {
				t.Fatalf("upsert failed: got %d", stats.Succeeded)
			}
		})
	}
}

func TestUncrawled(t *testing.T) {
	type want struct {
		images []*Image
		err    error
	}

	for _, c := range []struct {
		name   string
		status int
		resp   string
		want
	}{
		{
			name:   "basic",
			status: http.StatusCreated,
			resp: `{
				"took": 2,
				"timed_out": false,
				"_shards": {
				  "total": 5,
				  "successful": 5,
				  "skipped": 0,
				  "failed": 0
				},
				"hits": {
				  "total": 27877,
				  "max_score": 1,
				  "hits": [
					{
					  "_index": "test-images",
					  "_type": "image",
					  "_id": "http://www.example.com/path/to/nowhere",
					  "_score": 1,
					  "_source": {
						"id": "http://www.example.com/path/to/nowhere",
						"alt": "some image we have here",
						"nsfw_score": 0.0002,
						"crawled": "20180520"
					  }
					}
				  ]
				}
			  }`,
			want: want{
				images: []*Image{
					{
						ID:      "http://www.example.com/path/to/nowhere",
						Alt:     "some image we have here",
						NSFW:    0.0002,
						Crawled: "20180520",
					},
				},
				err: nil,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			handler := http.NotFound
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler(w, r)
			}))
			defer ts.Close()

			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.resp))
			}

			e, err := MockService(ts.URL)
			if err != nil {
				t.Fatal(err)
			}

			got, err := e.Uncrawled(100)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, c.want.images) {
				t.Fatalf("got %+v; want %+v", got, c.want.images)
			}
		})
	}
}

func TestSetup(t *testing.T) {
	for _, c := range []struct {
		name   string
		status int
		resp   string
	}{
		{
			name:   "ok",
			status: http.StatusOK,
			resp:   `{"acknowledged": true}`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			handler := http.NotFound
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler(w, r)
			}))
			defer ts.Close()

			handler = func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
				w.Write([]byte(c.resp))
			}

			e, err := MockService(ts.URL)
			if err != nil {
				t.Fatal(err)
			}

			if err := e.Setup(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func MockService(url string) (*ElasticSearch, error) {
	client, err := elastic.NewSimpleClient(elastic.SetURL(url))
	if err != nil {
		return nil, err
	}

	bulk, err := client.BulkProcessor().Stats(true).Do(context.TODO())
	if err != nil {
		return nil, err
	}

	return &ElasticSearch{
		Client: client,
		Index:  "images",
		Type:   "image",
		Bulk:   bulk,
	}, nil
}
