package crawler

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jivesearch/jivesearch/search/document"
	img "github.com/jivesearch/jivesearch/search/image"

	"github.com/jarcoal/httpmock"
	"github.com/jivesearch/jivesearch/search/crawler/robots"
	"github.com/spf13/pflag"
)

func TestNew(t *testing.T) {
	now = func() time.Time {
		t, _ := time.Parse(time.RFC3339, "2017-09-01T15:04:05Z")
		return t
	}

	p := &mockProvider{
		m: make(map[string]interface{}),
	}

	p.SetDefault("crawler.useragent.full", "test-bot-full")
	p.SetDefault("crawler.useragent.short", "test-bot-short")
	p.SetDefault("crawler.workers", 10)
	p.SetDefault("crawler.seeds", []string{"http://example.com", "https://another.com"})
	p.SetDefault("crawler.since", 45*24*time.Hour)
	p.SetDefault("crawler.max.queue.links", 100000)
	p.SetDefault("crawler.max.links", 10)
	p.SetDefault("crawler.max.domain.links", 100)
	p.SetDefault("crawler.truncate.title", 100)
	p.SetDefault("crawler.truncate.keywords", 25)
	p.SetDefault("crawler.truncate.description", 250)
	p.SetDefault("crawler.max.bytes", 10240000) // 10MB

	want := &Crawler{
		HTTPClient: http.DefaultClient,
		UserAgent: UserAgent{
			Full:  "test-bot-full",
			Short: "test-bot-short",
		},
		workers:        10,
		seeds:          []string{"http://example.com", "https://another.com"},
		since:          45 * 24 * time.Hour,
		maxQueueLinks:  100000,
		maxLinks:       10,
		maxDomainLinks: 100,
		maxBytes:       10240000,
		truncate: truncate{
			title:       100,
			keywords:    25,
			description: 250,
		},
		wg: sync.WaitGroup{},
		stats: &Stats{
			Start:       time.Date(2017, time.September, 01, 15, 4, 5, 0, time.UTC),
			StatusCodes: make(map[int]int64),
		},
	}

	got := New(p)
	got.channels = channels{}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

type mockProvider struct {
	m map[string]interface{}
}

func (p *mockProvider) SetDefault(key string, value interface{}) {
	p.m[key] = value
}
func (p *mockProvider) SetTypeByDefaultValue(bool) {}
func (p *mockProvider) BindPFlag(key string, flg *pflag.Flag) error {
	return nil
}
func (p *mockProvider) Get(key string) interface{} {
	return p.m[key]
}
func (p *mockProvider) GetString(key string) string {
	return p.m[key].(string)
}
func (p *mockProvider) GetInt(key string) int {
	return p.m[key].(int)
}
func (p *mockProvider) GetStringSlice(key string) []string {
	return p.m[key].([]string)
}

var seeds = []string{"http://example.com", "https://another.com"}

func TestStart(t *testing.T) {
	c := &Crawler{
		HTTPClient: http.DefaultClient,
		UserAgent: UserAgent{
			Full:  "test-bot-full",
			Short: "test-bot-short",
		},
		workers:        10,
		seeds:          seeds,
		since:          45 * 24 * time.Hour,
		maxLinks:       10,
		maxDomainLinks: 100,
		truncate: truncate{
			title:       100,
			keywords:    25,
			description: 250,
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

	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, s := range c.seeds {
		responder := httpmock.NewStringResponder(
			200,
			"hello",
		)
		httpmock.RegisterResponder("GET", s, responder)
	}

	c.Queue = &mockQueue{}
	c.Backend = &mockBackend{}
	defer c.Close()

	if err := c.Start(1 * time.Second); err != nil {
		t.Fatal(err)
	}

	httpmock.Reset()
}

func TestWork(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	for _, c := range []struct {
		name   string
		lnk    string
		rbts   string
		cached bool
		body   string
	}{
		{
			name:   "cached robots",
			lnk:    "https://www.example1.com",
			rbts:   `User-agent: *\nAllow: /`,
			cached: true,
			body:   `hello world`,
		},
		{
			name:   "not cached robots",
			lnk:    "https://www.example2.com",
			rbts:   `User-agent: *\nAllow: /`,
			cached: false,
			body:   `hello world`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			cr := &Crawler{
				HTTPClient: http.DefaultClient,
				UserAgent: UserAgent{
					Full:  "test-bot-full",
					Short: "test-bot-short",
				},
				workers:        10,
				seeds:          seeds,
				since:          45 * 24 * time.Hour,
				maxLinks:       10,
				maxDomainLinks: 100,
				truncate: truncate{
					title:       100,
					keywords:    25,
					description: 250,
				},
				channels: channels{
					links:  make(chan string),
					ch:     make(chan string),
					cancel: make(chan bool),
					err:    make(chan error),
				},
				wg:    sync.WaitGroup{},
				stats: &Stats{Start: now(), StatusCodes: make(map[int]int64)},
			}

			cr.Queue = &mockQueue{}
			cr.Backend = &mockBackend{}
			cr.Robots = &MockRobotsCache{m: make(map[string]*robots.Robots)}

			u, err := url.Parse(c.lnk)
			if err != nil {
				t.Fatalf("expected nil error; got %v", err)
			}

			// cache a rbts txt file
			if c.cached {
				rbts := robots.New(u.Scheme + "://" + u.Host).
					SetStatusCode(200).
					SetExpires()

				if err := rbts.SetBody(ioutil.NopCloser(bytes.NewReader([]byte(c.rbts)))); err != nil {
					t.Fatalf("expected nil error; got %v", err)
				}

				cr.Robots.Put(rbts)
			}

			httpmock.RegisterResponder(
				"GET",
				u.ResolveReference(RobotsPath).String(),
				httpmock.NewStringResponder(200, c.rbts),
			)

			httpmock.RegisterResponder(
				"GET",
				c.lnk,
				httpmock.NewStringResponder(200, c.body),
			)

			cr.work(c.lnk)
		})

		httpmock.Reset()
	}

	httpmock.Reset()
}

func TestCalculateHostDelay(t *testing.T) {
	type retryAfter struct {
		value  string
		format string
	}

	for _, c := range []struct {
		name   string
		status int
		delay  time.Duration
		now    time.Time
		retryAfter
		raFormat string
		want     time.Duration
		err      error
	}{
		{
			name:   "200",
			status: 200,
			delay:  10 * time.Second,
			want:   10 * time.Second,
			err:    nil,
		},
		{
			name:   "error fetching robots",
			status: -1,
			delay:  600 * time.Second,
			want:   600 * time.Second,
			err:    nil,
		},
		{
			name:   "not crawled",
			status: -1,
			want:   0 * time.Second,
			err:    nil,
		},
		{
			name:   "negative crawl delay",
			status: 200,
			delay:  -4 * time.Second,
			want:   1 * time.Second,
			err:    nil,
		},
		{
			name:   "5xx error",
			status: 500,
			delay:  2 * time.Second,
			want:   10 * time.Minute,
			err:    nil,
		},
		{
			name:       "retry after (integer)",
			status:     200,
			delay:      2 * time.Second,
			retryAfter: retryAfter{"30", ""},
			want:       30 * time.Second,
			err:        nil,
		},
		{
			name:       "retry after (datetime)",
			status:     200,
			delay:      2 * time.Second,
			now:        time.Date(2017, time.August, 14, 15, 3, 5, 0, time.UTC),
			retryAfter: retryAfter{"Mon, 14 Aug 2017 15:04:05 GMT", time.RFC1123},
			raFormat:   time.RFC1123,
			want:       60 * time.Second,
			err:        nil,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.retryAfter.format != "" {
				now = func() time.Time {
					return c.now
				}
			}

			got := calculateHostDelay(c.status, c.retryAfter.value, c.delay)

			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %q; want %q", got, c.want)
			}
		})
	}
}

type mockQueue struct{}

func (q *mockQueue) AddLink(lnk string) error {
	return nil
}

func (q *mockQueue) CountLinks() (int64, error) {
	return 100, nil
}

func (q *mockQueue) QueueLink(time.Duration) (string, error) {
	return "", nil
}

func (q *mockQueue) ReserveHost(host string, ttl time.Duration) error {
	return nil
}

func (q *mockQueue) DelayHost(host string, ttl time.Duration) error {
	return nil
}

func (q *mockQueue) Delete(lnks []string) error {
	return nil
}

type mockBackend struct{}

func (m *mockBackend) Setup() error {
	return nil
}

func (m *mockBackend) CrawledAndCount(u, domain string) (time.Time, int, error) {
	return time.Date(2016, time.August, 14, 15, 3, 5, 0, time.UTC), 10, nil
}

func (m *mockBackend) Upsert(*document.Document) error {
	return nil
}

type MockRobotsCache struct {
	sync.Mutex
	m map[string]*robots.Robots
}

func (c *MockRobotsCache) IndexExists() (bool, error) {
	return true, nil
}

func (c *MockRobotsCache) Setup() error {
	return nil
}

func (c *MockRobotsCache) Put(r *robots.Robots) {
	c.Lock()
	c.m[r.SchemeHost] = r
	c.Unlock()
}
func (c *MockRobotsCache) Get(sh string) (*robots.Robots, error) {
	c.Lock()
	defer c.Unlock()

	val, ok := c.m[sh]
	if !ok {
		return &robots.Robots{}, nil
	}

	val.Cached = true
	return val, nil
}
