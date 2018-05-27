package image

import (
	"fmt"
	"net/url"
)

// Image is a link to an image
type Image struct {
	ID        string  `json:"id"`
	Alt       string  `json:"alt,omitempty"`
	NSFW      float64 `json:"nsfw_score,omitempty"`
	Copyright string  `json:"copyright,omitempty"`
	Height    int     `json:"height,omitempty"`
	Width     int     `json:"width,omitempty"`
	Crawled   string  `json:"crawled,omitempty"`
}

var errInvalidURL = fmt.Errorf("invalid url")

// New creates a new *Image and validates the url
func New(src string) (*Image, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}

	if u.String() == "" {
		return nil, errInvalidURL
	}

	return &Image{ID: u.String()}, nil
}
