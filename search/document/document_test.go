package document

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	img "github.com/jivesearch/jivesearch/search/image"
	"golang.org/x/text/language"
)

func TestNew(t *testing.T) {
	type want struct {
		document *Document
		err      error
	}

	for _, c := range []struct {
		name string
		link string
		want
	}{
		{
			name: "uppercase letters", // the scheme, domain, and host should be lowercase. Everything else can remain.
			link: "htTp://WWW.eXamplE.cOm/This-Part-Can-Be/UpperCase/or/lowercase?And=a+QuerY",
			want: want{
				&Document{
					ID:        "http://www.example.com/This-Part-Can-Be/UpperCase/or/lowercase?And=a+QuerY",
					Scheme:    "http",
					Host:      "www.example.com",
					Domain:    "example.com",
					TLD:       "com",
					PathParts: "This Part Can Be UpperCase or lowercase",
				},
				nil,
			},
		},
		{
			name: "ftp",
			link: "ftp://news.example.org/news/world",
			want: want{
				nil,
				errInvalidScheme,
			},
		},
		{
			name: "ignore fragment",
			link: "https://example.com/pagina/#something",
			want: want{
				&Document{
					ID:        "https://example.com/pagina/",
					Scheme:    "https",
					Host:      "example.com",
					Domain:    "example.com",
					TLD:       "com",
					PathParts: "pagina",
				},
				nil,
			},
		},
		{
			name: "weird characters",
			link: "https://api.example.co.uk/path<s/t#his[/?q=that&p=#that",
			want: want{
				&Document{
					ID:        "https://api.example.co.uk/path%3Cs/t",
					Scheme:    "https",
					Host:      "api.example.co.uk",
					Domain:    "example.co.uk",
					TLD:       "uk",
					PathParts: "path<s t",
				},
				nil,
			},
		},
		{
			name: "relative link",
			link: "/path/somewhere?and=query",
			want: want{
				nil,
				errInvalidScheme,
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := New(c.link)
			if !reflect.DeepEqual(err, c.want.err) {
				t.Fatalf("got error %q; want %q", err, c.want.err)
			}

			if got != nil {
				got.URL = nil
			}

			if !reflect.DeepEqual(got, c.want.document) {
				t.Fatalf("got %+v; want: %+v", got, c.want.document)
			}
		})
	}
}

func TestSchemeHost(t *testing.T) {
	for _, c := range []struct {
		name string
		link string
		want string
	}{
		{"simple", "http://api.example.com", "http://api.example.com"},
		{"complex", "https://example.com/some/path/?and=query#fragment", "https://example.com"},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := New(c.link)
			if err != nil {
				t.Fatalf("expected nil error; got %q", err)
			}

			got := d.SchemeHost()

			if got != c.want {
				t.Fatalf("got %v; want: %v", got, c.want)
			}
		})
	}
}

func TestSetStatusCode(t *testing.T) {
	for _, code := range []int{
		200, 0, 100, 404, 500,
	} {
		t.Run(fmt.Sprintf("code: %d", code), func(t *testing.T) {
			d := &Document{}
			d.SetStatusCode(code)
			got := d.StatusCode

			if got != code {
				t.Fatalf("got %+v; want: %+v", got, code)
			}
		})
	}
}

func TestSetCrawled(t *testing.T) {
	for _, c := range []struct {
		tme  time.Time
		want string
	}{
		{time.Date(2017, 7, 24, 23, 0, 0, 0, time.UTC), "20170724"},
		{time.Date(1996, 12, 10, 4, 54, 32, 72, time.Local), "19961210"},
	} {
		t.Run(fmt.Sprintf("date: %v", c.tme), func(t *testing.T) {
			d := &Document{}
			d.SetCrawled(c.tme)
			if d.Crawled != c.want {
				t.Fatalf("got %+v; want: %+v", d.Crawled, c.want)
			}
		})
	}
}

func TestSetHeader(t *testing.T) {
	for _, c := range []struct {
		name string
		h    http.Header
	}{
		{
			"basic",
			http.Header{
				"Accept-Language": []string{"en-US"},
				"Cache-Control":   []string{"no-cache"},
				"Link":            []string{`<http://www.example.com/canonical>; rel="canonical"`},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := &Document{}
			d.SetHeader(c.h)

			if !reflect.DeepEqual(d.header, c.h) {
				t.Fatalf("got %+v; want: %+v", d.header, c.h)
			}
		})
	}
}

func TestSetCanonical(t *testing.T) {
	for _, c := range []struct {
		name string
		id   string
		lnk  string
		want bool
	}{
		{"false", "https://www.example.com", "http://www.example.com", false},
		{"true", "http://www.example.com", "http://www.example.com", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := Document{
				ID: c.id,
				header: http.Header{
					"Link": []string{
						fmt.Sprintf(`<%v>; rel="canonical"`, c.lnk),
					},
				},
			}
			ch := make(chan string)
			go func(ch chan string) {
				<-ch
			}(ch)

			d.SetCanonical(ch)

			if d.Canonical != c.want {
				t.Fatalf("got %+v; want: %+v", d.Canonical, c.want)
			}
		})
	}
}

func TestSetPolicyFromHeader(t *testing.T) {
	for _, c := range []struct {
		name   string
		bot    string
		policy []string
		want   Policy
	}{
		{"default", "", []string{""}, Policy{true, true}},
		{"none", "", []string{"none"}, Policy{false, false}},
		{"conflicting policies", "", []string{"all", "noindex, nofollow"}, Policy{false, false}},
		{"conflicting policies2", "", []string{"all", "nofollow"}, Policy{true, false}},
		{"conflicting policies3", "", []string{"all", "noindex"}, Policy{false, true}},
		{"conflicting policies4", "", []string{"noindex, nofollow", "all"}, Policy{false, false}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := Document{
				header: make(http.Header),
			}

			for _, p := range c.policy {
				d.header.Add("X-Robots-Tag", p)
			}

			d.SetPolicyFromHeader(c.bot)

			if !reflect.DeepEqual(d.Policy, c.want) {
				t.Fatalf("got %+v; want: %+v", d.Policy, c.want)
			}
		})
	}
}

func TestSetTokenizer(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string
	}{
		{
			"html",
			`<html><body>this is a body.</body></html>`,
			"text/html",
		},
		{
			"text",
			`This is a non-html body. Just a simple text body.`,
			"text/plain",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			d := Document{}

			err := d.SetTokenizer(strings.NewReader(c.body))
			if err != nil {
				t.Fatalf("expected nil error; got %q", err)
			}

			if d.MIME != c.want {
				t.Fatalf("got %+v; want: %+v", d.MIME, c.want)
			}
		})
	}
}

func TestSetContent(t *testing.T) {
	for _, c := range []struct {
		name                string
		url                 string
		status              int
		header              http.Header
		body                string
		links               []string
		maxLinks            int
		ch                  chan string
		images              chan *img.Image
		truncateTitle       int
		truncateKeywords    int
		truncateDescription int
		want                Content
	}{
		{
			name:   "basic",
			url:    "http://www.example.com",
			status: http.StatusOK,
			body: `<html>
					     <head>
						   <title>The title of a page</title>
						   <meta name="keywords" content="some keywords for a search engine"/><!--self closing-->
						   <meta name="description" content="A description of the content">
						 </head>
						 <body>
						   <a href="http://www.example.com/link/to/somewhere">A link</a>
						   <a href="http://www.example.com/donotfollow" rel="nofollow">Don't follow this link!</a>
						   <a href="http://www.example.com/link/to/somewhere/else">A link to somewhere else</a>
						 </body>
					   </html>`,
			links: []string{
				"http://www.example.com/link/to/somewhere",
				"http://www.example.com/link/to/somewhere/else",
			},
			maxLinks:            10,
			ch:                  make(chan string),
			truncateTitle:       100,
			truncateKeywords:    5,
			truncateDescription: 14,
			want: Content{
				StatusCode:  http.StatusOK,
				Language:    language.English,
				Title:       "The title of a page",
				Keywords:    "some keywords for a search",
				Description: "A description",
				Policy:      Policy{Index: true, follow: true},
			},
		},
		{
			name:   "language and policy",
			url:    "http://www.example.com",
			status: http.StatusOK,
			body: `<html lang="es">
						<head>
							<meta name="robots" content="noindex, nofollow">
							<meta name="robots" content="noindex, follow"><!-- conflicting policies...follow most restrictive -->
						</head>
						<body>
							<a href="http://www.example.com/link/to/somewhere">A link</a>
						</body>
					</html>`,
			links:               []string{},
			maxLinks:            10,
			ch:                  make(chan string),
			truncateTitle:       100,
			truncateKeywords:    5,
			truncateDescription: 14,
			want: Content{
				StatusCode:  http.StatusOK,
				Language:    language.Spanish,
				Title:       "",
				Keywords:    "",
				Description: "",
				Policy:      Policy{Index: false, follow: false},
			},
		},
		{
			name:   "canonical link",
			url:    "https://example.com",
			status: http.StatusOK,
			body: `<html>
				     <head>
					   <title>The title of a page</title>
					   <meta name="keywords" content="some keywords for a search engine"/><!--self closing-->
					   <meta name="description" content="A description of the content">
					   <link rel="canonical" href="https://example.com/canonical.php" />
					 </head>
					 <body>
					   <a href="http://www.example.com/link/to/somewhere">A link</a><!--not collected-->
					</body>
				   </html>`,
			links: []string{
				"https://example.com/canonical.php",
				"http://www.example.com/link/to/somewhere",
			},
			maxLinks:            10,
			ch:                  make(chan string),
			truncateTitle:       100,
			truncateKeywords:    5,
			truncateDescription: 14,
			want: Content{
				StatusCode:  http.StatusOK,
				canonical:   "https://example.com/canonical.php",
				Language:    language.English,
				Title:       "The title of a page",
				Keywords:    "some keywords for a search",
				Description: "A description",
				Policy:      Policy{Index: true, follow: true},
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			collected := make(chan []string)

			go func() {
				lnks := []string{}
				for lnk := range c.ch {
					lnks = append(lnks, lnk)
				}
				collected <- lnks
			}()

			d, err := New(c.url)
			if err != nil {
				t.Fatalf("expected nil error; got %q", err)
			}

			resp := &http.Response{
				StatusCode:    c.status,
				Body:          ioutil.NopCloser(strings.NewReader(c.body)),
				ContentLength: int64(len(c.body)),
				Header:        c.header,
			}

			defer resp.Body.Close()

			cpy := &Document{}
			*cpy = *d

			err = d.SetHeader(resp.Header).
				SetPolicyFromHeader("").
				SetStatusCode(c.status).
				SetTokenizer(resp.Body)

			if err != nil {
				t.Fatalf("expected nil error; got %q", err)
			}

			err = d.SetContent("", c.maxLinks, c.ch, c.images,
				c.truncateTitle, c.truncateKeywords, c.truncateDescription)

			if err != nil {
				t.Fatalf("expected nil error; got %q", err)
			}

			close(c.ch)
			got := <-collected

			if !reflect.DeepEqual(got, c.links) {
				t.Fatalf("got %v links; want %v", got, c.links)
			}

			if !reflect.DeepEqual(d.Content, c.want) {
				t.Fatalf("got %+v; want: %+v", d.Content, c.want)
			}

			// make sure SetContent changed no other part of the doc
			// (this also checks that New() doesn't change Content)
			d.tokenizer, d.MIME, d.Content = nil, "", Content{}
			if !reflect.DeepEqual(d, cpy) {
				t.Fatalf("Parse() changed parts outside of the `Content`: got %+v; want: %+v", d, cpy)
			}
		})
	}
}

func TestLanguages(t *testing.T) {
	for _, c := range []struct {
		name string
		arg  []language.Tag
		want []language.Tag
	}{
		{"basic", []language.Tag{}, available},
		{"en", []language.Tag{language.English}, available},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := Languages(c.arg)

			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}
