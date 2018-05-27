// Package crawler is a distributed web crawler.
package crawler

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jivesearch/jivesearch/search/document"
	img "github.com/jivesearch/jivesearch/search/image"

	"github.com/jivesearch/jivesearch/config"
	"github.com/jivesearch/jivesearch/log"
	"github.com/jivesearch/jivesearch/search/crawler/queue"
	"github.com/jivesearch/jivesearch/search/crawler/robots"
	"github.com/pkg/errors"
	"github.com/temoto/robotstxt"

	"sync"
	"time"
)

// Crawler holds crawler settings for our UserAgent, Seed URLs, etc.
type Crawler struct {
	HTTPClient *http.Client
	UserAgent
	workers        int
	seeds          []string
	since          time.Duration // how often we crawl a page
	maxBytes       int64         // max number of bytes of doc to download...-1 for no limit
	maxQueueLinks  int64         // max links for our queue
	maxLinks       int           // max links to extract from a document
	maxDomainLinks int           // max links to store for a domain by default (votes will increase this)
	truncate
	Robots robots.Cacher
	Queue  queue.Queuer
	channels
	wg    sync.WaitGroup
	stats *Stats
	Backend
	ImageBackend
}

type channels struct {
	links  chan string
	images chan *img.Image
	ch     chan string
	cancel chan bool
	err    chan error
}

// UserAgent holds the full and short version of the crawler's useragent
type UserAgent struct {
	Full  string
	Short string
}

type truncate struct {
	title       int // chars
	keywords    int // words
	description int // chars
}

// Backend outlines methods to save documents and count the docs a domain has
type Backend interface {
	Setup() error
	CrawledAndCount(u, domain string) (time.Time, int, error) // gotta be a better name for this
	Upsert(*document.Document) error
}

// ImageBackend outlines methods to save image links
type ImageBackend interface {
	Setup() error
	Upsert(*img.Image) error
}

var now = func() time.Time { return time.Now().UTC() }

// RobotsPath is robots.txt path
var RobotsPath, _ = url.Parse("/robots.txt")

// New creates a Crawler from a config Provider
func New(cfg config.Provider) *Crawler {
	return &Crawler{
		HTTPClient: http.DefaultClient,
		UserAgent: UserAgent{
			Full:  cfg.GetString("crawler.useragent.full"),
			Short: cfg.GetString("crawler.useragent.short"),
		},
		workers:        cfg.GetInt("crawler.workers"),
		seeds:          cfg.GetStringSlice("crawler.seeds"),
		since:          cfg.Get("crawler.since").(time.Duration),
		maxBytes:       int64(cfg.GetInt("crawler.max.bytes")),
		maxQueueLinks:  int64(cfg.GetInt("crawler.max.queue.links")),
		maxLinks:       cfg.GetInt("crawler.max.links"),
		maxDomainLinks: cfg.GetInt("crawler.max.domain.links"),
		truncate: truncate{
			title:       cfg.GetInt("crawler.truncate.title"),
			keywords:    cfg.GetInt("crawler.truncate.keywords"),
			description: cfg.GetInt("crawler.truncate.description"),
		},
		channels: channels{
			links:  make(chan string),
			images: make(chan *img.Image),
			ch:     make(chan string),
			cancel: make(chan bool),
			err:    make(chan error),
		},
		wg:    sync.WaitGroup{},
		stats: &Stats{Start: now(), StatusCodes: make(map[int]int64)},
	}
}

// Start the crawler
func (c *Crawler) Start(t time.Duration) error {
	ctx, cancel := context.WithTimeout(context.TODO(), t)
	defer cancel()

	go c.linkHandler()
	go c.imageHandler()
	go c.startQueue()

	go func() {
		for _, lnk := range c.seeds {
			c.links <- lnk
		}

		for worker := 0; worker < c.workers; worker++ {
			c.wg.Add(1)
			go func(w int) {
				defer c.wg.Done()

				for lnk := range c.ch {
					c.work(lnk)
				}
			}(worker)
		}
	}()

	var err error

	select {
	case <-ctx.Done():
	case err = <-c.err:
		return err
	}

	c.cancel <- true
	time.Sleep(1 * time.Second) // w/out we get a race condition in our tests (code smell)
	c.wg.Wait()
	close(c.links)
	close(c.images)

	return err
}

func (c *Crawler) linkHandler() {
	for lnk := range c.links {
		if err := c.Queue.AddLink(lnk); err != nil {
			c.err <- errors.Wrapf(err, "%q", lnk)
			return
		}
	}
}

func (c *Crawler) imageHandler() {
	for img := range c.images {
		if err := c.ImageBackend.Upsert(img); err != nil {
			c.err <- errors.Wrapf(err, "unable to insert image: %v", img.ID)
			return
		}
	}
}

func (c *Crawler) startQueue() {
	for {
		select {
		case <-c.cancel:
			close(c.ch)
			return
		default:
			// Note: link s/b in queue for >= refresh interval of crawler's backend
			// Alternative is to keep track of items queued and delete them in bulk's afterFunction
			lnk, err := c.Queue.QueueLink(600 * time.Second)
			if err != nil {
				c.err <- errors.Wrapf(err, "%q", lnk)
				return
			}

			if lnk != "" {
				c.ch <- lnk
			}
		}
	}
}

func (c *Crawler) work(lnk string) {
	doc, err := document.New(lnk)
	if err != nil {
		log.Debug.Println(errors.Wrapf(err, "link: %q", lnk))
		return
	}

	sh := doc.SchemeHost()

	if err := c.Queue.ReserveHost(sh, 600*time.Second); err != nil {
		msg := errors.Wrapf(err, "host: %q", sh)
		switch err {
		case queue.ErrAlreadyReserved:
			//log.Debug.Println(msg)
		default:
			c.err <- msg
		}

		return
	}

	var delay time.Duration
	var ra string // Retry-After header

	defer func() {
		delay = calculateHostDelay(doc.StatusCode, ra, delay)
		if err := c.Queue.DelayHost(sh, delay); err != nil {
			c.err <- errors.Wrapf(err, "host: %q, delay: %q", sh, delay)
		}
	}()

	crawled, cnt, err := c.Backend.CrawledAndCount(doc.ID, doc.Domain)
	if err != nil {
		c.err <- errors.Wrapf(err, doc.ID)
		return
	}

	// crawled recently...always skip
	if !crawled.Before(now().Add(-c.since)) {
		return
	}

	// new doc? only crawl if we have room for that domain
	// TODO: make count dependent on votes
	if crawled == (time.Time{}) && cnt > c.maxDomainLinks {
		return
	}

	doc.SetStatusCode(-1).SetCrawled(now())

	rbt := c.fetchRobots(doc)
	rbtsText, err := robotstxt.FromStatusAndString(rbt.StatusCode, rbt.Body)
	if err != nil {
		delay = 600 * time.Second
		return
	}

	group := rbtsText.FindGroup(c.UserAgent.Full)
	if !group.Test(doc.URL.Path) {
		return
	}

	delay = group.CrawlDelay

	resp, err := c.doRequest(doc.ID)
	if err != nil {
		log.Info.Println(err)
		return
	}

	defer resp.Body.Close()

	c.stats.Update(resp.StatusCode)
	doc.SetStatusCode(resp.StatusCode)
	ra = resp.Header.Get("Retry-After")

	if doc.StatusCode == http.StatusOK {
		var b io.Reader = resp.Body
		if c.maxBytes > -1 {
			b = io.LimitReader(b, c.maxBytes)
		}

		err = doc.SetHeader(resp.Header).
			SetPolicyFromHeader(c.UserAgent.Short).
			SetTokenizer(b)

		if err != nil {
			log.Debug.Printf("document parsing error: %v\n%v", doc.ID, err)
			return
		}

		// TODO: image (& video?) search and extract some of the text of pdf files.
		// Note: some html is mismarked as text/xml
		if doc.MIME != "text/plain" && doc.MIME != "text/html" && doc.MIME != "text/xml" {
			return
		}

		queueCnt, err := c.Queue.CountLinks()
		if err != nil {
			log.Debug.Printf("unable to count links in queue: %v\n%v", doc.ID, err)
			return
		}

		maxLinks := c.maxLinks
		if queueCnt > c.maxQueueLinks {
			maxLinks = 0
		}

		if err := doc.SetContent(c.UserAgent.Short, maxLinks, c.links, c.images,
			c.truncate.title, c.truncate.keywords, c.truncate.description); err != nil {
			log.Debug.Printf("document parsing error: %v\n%v", doc.ID, err)
		}

		// don't index content if not wanted or if not canonical
		if doc.SetCanonical(c.links); !doc.Canonical || !doc.Index {
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

	if err := c.Backend.Upsert(doc); err != nil {
		c.err <- errors.Wrapf(err, "unable to insert doc: %v", doc.ID)
		return
	}
}

// fetchRobots fetches and caches the robots.txt file
func (c *Crawler) fetchRobots(doc *document.Document) *robots.Robots {
	sh := doc.SchemeHost()
	rbt, err := c.Robots.Get(sh)
	if err != nil {
		c.err <- errors.Wrapf(err, "cannot get robots.txt from cache for %v", sh)
		return rbt
	}

	// check if the robots are expired
	var expired bool
	if rbt.Cached {
		expired, err = rbt.Expired()
		if err != nil {
			c.err <- errors.Wrapf(err, "unable to determine expiration for cached robots.txt %v", sh)
			return rbt
		}
	}

	if !rbt.Cached || expired {
		u := doc.URL.ResolveReference(RobotsPath)
		resp, err := c.doRequest(u.String())
		if err != nil {
			log.Info.Println(err)
			return rbt
		}

		defer resp.Body.Close()

		rbt = robots.New(sh).
			SetStatusCode(resp.StatusCode).
			SetExpires()

		// We only need the body if we have a 2xx response
		// All other statuses don't need to parse the body
		// 4xx response is allow all. 5xx is disallow all.
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if err := rbt.SetBody(resp.Body); err != nil {
				log.Debug.Println(errors.Wrapf(err, "error in reading robots.txt file for %v", sh))
				return rbt
			}
		}

		c.Robots.Put(rbt)
	}

	return rbt
}

func calculateHostDelay(status int, retry string, delay time.Duration) time.Duration {
	max := func(x, y time.Duration) time.Duration {
		if x > y {
			return x
		}
		return y
	}

	// we take the greater of robots.txt crawl-delay, replay-after header, or a hard 10 minutes
	// in case of a 5xx error.
	if retry != "" && (status < 300 || status > 399) {
		// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After
		// The "Retry-After" header applies to 503 error (server temporarily unavailable), a back-off
		// policy, or even a wait-time in between a redirect. Redirects imply a per-link policy
		// while non-redirects implies a site-wide policy (which we are interested in).
		// http://www.w3.org/Protocols/rfc2616/rfc2616-sec3.html#sec3.3.1
		// RA can be in seconds or a date (RFC1123 preferred). All times MUST be in GMT per standard.
		// We also consider the widely-used RFC3339 (eg ISO 8601).

		// First, try to parse as integer. If error, parse as datetime.
		i, err := strconv.Atoi(retry)
		if err != nil {
			for _, f := range []string{
				time.RFC1123, time.RFC1123Z, time.RFC822, time.RFC822Z,
				time.RFC850, time.ANSIC, time.RFC3339,
			} {
				if d, err := time.Parse(f, retry); err == nil {
					i = int(d.Sub(now()).Seconds())
					break
				}
			}
		}
		delay = max(time.Duration(i)*time.Second, delay)
	}

	switch {
	case status >= 500 && status < 600: // server error
		// TODO: implement an exponential backoff policy here
		// for now we will be conservative & implement a hard 10-min delay
		// for each server error.
		delay = max(600*time.Second, delay)
	case status == -1: // not crawled, remove the delay (but might be error fetching robots.txt)
		delay = max(0*time.Second, delay)
	case delay < 1*time.Second: // min of 1 second if crawled
		delay = 1 * time.Second
	}

	return delay
}

func (c *Crawler) doRequest(u string) (*http.Response, error) {
	// Note: Transport automatically adds "Accept-Encoding: gzip"
	// and transparently decodes response UNLESS you manually
	// set the "Accept-Encoding" header.
	// https://golang.org/src/net/http/transport.go#L131
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.UserAgent.Full)
	return c.HTTPClient.Do(req)
}

// Close the crawler
func (c *Crawler) Close() {
	log.Info.Println(c.stats.Elapsed().String())
}
