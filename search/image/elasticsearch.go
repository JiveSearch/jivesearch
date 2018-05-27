package image

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jivesearch/jivesearch/log"
	"github.com/olivere/elastic"
)

// ElasticSearch hold connection and index settings
type ElasticSearch struct {
	Client *elastic.Client
	Index  string
	Type   string
	Bulk   *elastic.BulkProcessor
}

// Upsert updates an image link or inserts it if it doesn't exist
// NOTE: Elasticsearch has a 512-byte limit on an insert operation.
// Upsert does not have that limit.
func (e *ElasticSearch) Upsert(img *Image) error {
	item := elastic.NewBulkUpdateRequest().
		Index(e.Index).
		Type(e.Type).
		Id(img.ID).
		DocAsUpsert(true).
		Doc(img)

	e.Bulk.Add(item)
	return nil
}

// Uncrawled finds images that haven't been crawled recently/yet
func (e *ElasticSearch) Uncrawled(number int) ([]*Image, error) {
	q := fmt.Sprintf(`{
		"query": {
			"bool": {
				"should": [
					{
						"bool": { 
							"must_not":[{"exists": {"field":"crawled"}}]
						}
					},
					{
						"bool":{
						"filter":[{ "range": {"crawled": {"lte": 19630301}}}]
						}
					}
				]
			}
		},
		"size": %v
	}`, number)

	images := []*Image{}

	res, err := e.Client.Search(e.Index).Source(q).Do(context.TODO())
	if err != nil {
		return images, err
	}

	for _, h := range res.Hits.Hits {
		img := &Image{}
		err := json.Unmarshal(*h.Source, img)
		if err != nil {
			return images, err
		}

		img.ID = h.Id
		images = append(images, img)
	}

	return images, nil
}

// Setup will create our image index
func (e *ElasticSearch) Setup() error {
	exists, err := e.Client.IndexExists(e.Index).Do(context.TODO())
	if err != nil {
		return err
	}

	if !exists {
		log.Info.Println("Creating index:", e.Index)
		if _, err = e.Client.CreateIndex(e.Index).Body(e.mapping()).Do(context.TODO()); err != nil {
			return err
		}
	}

	return nil
}

// mapping is the mapping of our image Index.
func (e *ElasticSearch) mapping() string {
	m := `{
		"mappings": {
			"image": {
				"_all": {
					"enabled": false
				},
				"dynamic": "strict",
				"properties": {
					"id": {
						"type": "text"
					},
					"alt": {
						"type": "text"
					},
					"nsfw_score": {
						"type": "double"
					},
					"crawled": {
						"type": "date",
						"format": "basic_date"
					}
				}
			}
		}
	}`

	return m
}
