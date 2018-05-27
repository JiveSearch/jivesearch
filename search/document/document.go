// Package document parses URLs and the HTML of a webpage
package document

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	img "github.com/jivesearch/jivesearch/search/image"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"
	"golang.org/x/net/publicsuffix"
	"golang.org/x/text/language"
)

var (
	errInvalidScheme = fmt.Errorf("invalid scheme")
)

// Document is the URL & parsed content of the page
// Note, since we want just a couple of fields from *url.URL
// (Scheme, Host) we explicitly set those. Much easier than
// a custom MarshalJSON method.
type Document struct {
	ID        string   `json:"id"` // store ID also as a field as sorting on document ID is not advised in Elasticsearch
	URL       *url.URL `json:"-"`
	Scheme    string   `json:"scheme,omitempty"`
	Host      string   `json:"host,omitempty"`       // not HostName()...we want the port for the robots.txt file
	Domain    string   `json:"domain,omitempty"`     // tld+1 -> example.com
	TLD       string   `json:"tld,omitempty"`        // com, org, uk, etc (we don't want co.uk just uk)
	PathParts string   `json:"path_parts,omitempty"` // https://api.example.com/path/to/something -> "path to something"
	Crawled   string   `json:"crawled,omitempty"`
	header    http.Header
	MIME      string `json:"mime,omitempty"`
	tokenizer *html.Tokenizer
	Content
	Votes int `json:"-"`
}

// Content is set from the response
type Content struct {
	StatusCode  int `json:"status,omitempty"`
	canonical   string
	Canonical   bool         `json:"canonical,omitempty"`
	Language    language.Tag `json:"-"`
	Date        string       `json:"date,omitempty"`
	Title       string       `json:"title,omitempty"`
	Keywords    string       `json:"keywords,omitempty"`
	Description string       `json:"description,omitempty"`
	Policy
}

// Policy tells us if we can index the content & store the links
type Policy struct {
	Index  bool `json:"index,omitempty"` // are we allowed to index the page?
	follow bool // are we allowed to follow links?
}

// New creates a new Document from a link and validates the url
func New(lnk string) (*Document, error) {
	u, err := ValidateURL(lnk)
	if err != nil {
		return nil, err
	}

	dom, err := ExtractDomain(u)
	if err != nil {
		return nil, err
	}

	tld := strings.Split(dom, ".")

	return &Document{
		ID:        u.String(),
		URL:       u,
		Scheme:    u.Scheme,
		Host:      u.Host,
		Domain:    dom,
		TLD:       tld[len(tld)-1],
		PathParts: path(u.Path),
	}, nil
}

// ValidateURL validates a link and returns a *url.URL
// Note: There seems to be a lot of overlap between this and handleLink()
func ValidateURL(lnk string) (*url.URL, error) {
	// we have to strip the fragment BEFORE we use ParseRequestURI
	u, err := url.Parse(lnk)
	if err != nil {
		return nil, err
	}

	u.Fragment = ""
	u, err = url.ParseRequestURI(u.String())
	if err != nil {
		return nil, err
	}

	// wrong scheme will also be filtered by handleLink ;)
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errInvalidScheme
	}

	u.Host = strings.ToLower(u.Host)
	return u, err
}

// ExtractDomain extracts the domain from a *url.URL
// e.g. "example.com" from "https://www.example.com/path/somewhere"
func ExtractDomain(u *url.URL) (string, error) {
	return publicsuffix.EffectiveTLDPlusOne(u.Host)
}

// SchemeHost simply concatenates the Scheme, '://', and Host
func (d *Document) SchemeHost() string {
	return d.URL.Scheme + "://" + d.URL.Host
}

func path(p string) string {
	path := strings.NewReplacer("/", " ", "-", " ").Replace(p)
	for _, ext := range []string{".html", ".htm", ".php"} {
		path = strings.TrimSuffix(path, ext)
	}
	s := strings.Fields(path)
	return strings.Join(removeDuplicates(s), " ")
}

// SetStatusCode sets the http status code
func (d *Document) SetStatusCode(code int) *Document {
	d.StatusCode = code
	return d
}

// SetCrawled marks the date the doc was crawled
func (d *Document) SetCrawled(t time.Time) *Document {
	d.Crawled = t.Format("20060102")
	return d
}

// SetHeader sets the Document's header to the response header.
func (d *Document) SetHeader(h http.Header) *Document {
	d.header = h
	return d
}

// SetPolicyFromHeader sets the indexing & follow policy of a document from the response header.
// A specific bot directive overrides a general robots directive (still TODO).
// We process the X-Robots-Tag header first so may not even get to the meta tag found in the html.
// https://developers.google.com/search/reference/robots_meta_tag
// https://stackoverflow.com/a/18330818/776942 (see end of answer)
// TODO: Process the bot directive.
func (d *Document) SetPolicyFromHeader(bot string) *Document {
	d.Policy = Policy{true, true} // assume we can index & follow unless proven otherwise

	// Get only returns the first value for a key...This version gets all values for a key.
	for key, values := range d.header {
		if c := http.CanonicalHeaderKey(key); c == "X-Robots-Tag" {
			for _, val := range values {
				d.setPolicy(bot, val)
			}
		}
	}

	return d
}

// TODO: Process the bot directive for our bot
// var botPolicy = regexp.MustCompile(`(?P<bot>.*):(?P<policy>.*)`)

// In case of competing directives we follow the most restrictive.
// Since our default is to Index and Follow we don't want to
// switch a "false" to "true" since we follow the most restrictive.
func (d *Document) setPolicy(bot, pol string) {
	for _, p := range strings.Split(pol, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		switch p {
		case "none":
			d.Policy.Index = false
			d.Policy.follow = false
		case "all": // see note above
		case "index": // see note above
		case "noindex":
			d.Policy.Index = false
		case "follow": // see note above
		case "nofollow":
			d.Policy.follow = false
		}
	}
}

// SetTokenizer sets the html tokenizer and MIME Type from the response's body (utf-8 encoded).
// It is the caller's responsibility to close the response body.
func (d *Document) SetTokenizer(b io.Reader) error {
	bdy := bufio.NewReader(b)
	peek, err := bdy.Peek(512) // DetectContentType needs just 512 bytes (sometimes less)
	if err != nil && err != io.EOF {
		return err
	}

	d.MIME = strings.Split(http.DetectContentType(peek), ";")[0]

	// html tokenizer requires utf-8
	utf, err := charset.NewReader(bdy, d.MIME)
	if err != nil {
		return err
	}

	d.tokenizer = html.NewTokenizer(utf)
	return nil
}

// SetContent parses the html and sets the language, title, description, extracts links, etc.
func (d *Document) SetContent(bot string, maxLinks int, links chan string, images chan *img.Image,
	truncateTitle, truncateKeywords, truncateDescription int) error {

	var collected int

	var tt html.TokenType
	var title bool

	for {
		tt = d.tokenizer.Next()

		switch tt {
		case html.ErrorToken:
			return nil
		case html.TextToken:
			if title {
				d.Title = d.extractText(string(d.tokenizer.Text()), truncateTitle)
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			t := d.tokenizer.Token()

			// Note: comparing DataAtom is faster (& uses less memory) than n.Data=="title", etc.
			switch t.DataAtom {
			case atom.Html:
				// A document may have multiple languages for different
				// sections of the <body>, <span>, etc. Since we only
				// extract text from the <head> section we
				// only need the language from the <html> tag.
				d.setLanguage(t)
				/*
					TODO: How to deal with rtl text (Arabic, Hebrew, etc)...simply reverse it?
					// Can we make use of MustParseScript in language package, which represents
					// ISO 15924 codes? ISO 15924 is mentioned in: https://en.wikipedia.org/wiki/Right-to-left
					dir, ok = getAttribute(t, "dir")
					if ok {
						dir = strings.ToLower(dir)
					}
				*/
			case atom.Link:
				if rel, _ := getAttribute(t, "rel"); rel == "canonical" {
					lnk, _ := getAttribute(t, "href")
					if lnk != d.ID {
						d.canonical = lnk
						links <- lnk
					}
				}
			case atom.Title:
				title = true
			case atom.Meta:
				if name, _ := getAttribute(t, "name"); name == "keywords" {
					if kw, ok := getAttribute(t, "content"); ok {
						wrds := strings.Replace(kw, ",", " ", -1)
						s := strings.Fields(wrds)
						s = removeDuplicates(s)
						if len(s) > truncateKeywords {
							s = s[:truncateKeywords]
						}
						d.Keywords = d.extractText(strings.Join(s, " "), -1)
					}
				}
				if name, _ := getAttribute(t, "name"); name == "description" {
					if des, ok := getAttribute(t, "content"); ok {
						d.Description = d.extractText(des, truncateDescription)
					}
				}
				name, _ := getAttribute(t, "name")
				// TODO: Like SetPolicyFromHeader(), we need to process bot directive
				if strings.EqualFold(name, "robots") || strings.EqualFold(name, bot) {
					content, _ := getAttribute(t, "content")
					d.setPolicy(bot, content)
				}
			case atom.A:
				if d.Policy.follow && (maxLinks == -1 || collected < maxLinks) {
					rel, _ := getAttribute(t, "rel")
					if !contains(strings.Fields(rel), "nofollow") {
						href, _ := getAttribute(t, "href")
						u, err := d.handleLink(href)
						if err == nil {
							links <- u
							collected++
						}
					}
				}
			case atom.Img:
				src, _ := getAttribute(t, "src")
				u, err := d.handleLink(src)
				if err != nil {
					continue
				}

				img, err := img.New(u)
				if err != nil {
					continue
				}

				img.Alt, _ = getAttribute(t, "alt")
				images <- img
			case atom.Time:
				// There are a few ways to get the creation date (or modified) date of the document:

				// the "created" or "last-modified" meta tags
				// <meta name="created" content="2009-05-09" />
				// <meta http-equiv="last-modified" content="Sat, 07 Apr 2001 00:58:08 GMT" />
				// The last-modified meta tag is NOT officially supported by html standards.

				// <time class="date" data-time="1485526583" datetime="2017-01-27T14:16:23+00:00">Jan 27, 2017 2:16 pm UTC</time>
				// https: //www.w3.org/TR/html51/infrastructure.html#dates-and-times

				// RDFa: https://alistapart.com/article/introduction-to-rdfa
				// <em property="dc:created" content="2009-05-09">
				// <em property="created" content="2009-05-09">

				// See: https://developers.google.com/search/docs/guides/intro-structured-data?visit_id=1-636402414686311906-557470069&rd=1

				// stackoverflow.com doesn't use any of these methods? How do other search engines get their creation date?
				// https://search.google.com/structured-data/testing-tool#url=https%3A%2F%2Fstackoverflow.com%2Fquestions%2F252703%2Fappend-vs-extend

			}
		case html.EndTagToken:
			t := d.tokenizer.Token()

			switch t.DataAtom {
			case atom.Title:
				title = false
			}
		}
	}
}

var canonicalHeader = regexp.MustCompile(`<(.*?)>; rel="canonical"`)

// SetCanonical sets Canonical to true if the Document's ID is the canonical URL
func (d *Document) SetCanonical(ch chan string) *Document {
	d.Canonical = true // assume it is unless proven otherwise

	// check the header (we check body in SetContent...header will override body)
	if lnk := d.header.Get("Link"); lnk != "" {
		m := canonicalHeader.FindStringSubmatch(strings.TrimSpace(lnk))
		if len(m) > 1 {
			d.canonical = m[1]
			ch <- m[1]
		}
	}

	if d.canonical != "" && d.canonical != d.ID {
		d.Canonical = false
	}

	return d
}

// remove duplicate words from a string
func removeDuplicates(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}

func contains(s []string, val string) bool {
	for _, a := range s {
		if strings.ToLower(strings.TrimSpace(a)) == val {
			return true
		}
	}
	return false
}

func (d *Document) handleLink(href string) (string, error) {
	var err = fmt.Errorf("invalid url")

	if len(href) < 3 || len(href) > 2083 {
		return "", err
	}

	// escape any invalid characters
	// need to check utf8.ValidString(href) afterwords??
	href, err = url.QueryUnescape(url.QueryEscape(href))
	if err != nil {
		return "", err
	}

	// maybe use url.ParseRequestURI instead????
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	u = d.URL.ResolveReference(u)
	if u.String() != d.ID && (u.Scheme == "http" || u.Scheme == "https") {
		return u.String(), nil
	}

	return "", err
}

func getAttribute(t html.Token, key string) (string, bool) {
	for _, a := range t.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

func (d *Document) extractText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if max != -1 && len(s) > max {
		s = s[:max]
	}

	return strings.TrimSpace(s)
}

func (d *Document) setLanguage(t html.Token) {
	tag := language.Tag{}

	if lang, _ := getAttribute(t, "lang"); lang != "" {
		tag = language.Make(strings.ToLower(lang))
	}

	d.Language, _, _ = Matcher.Match(tag) // we ignore the error
}

// Languages (will) verifies that languages are supported.
// An empty slice of supported languages implies you support every language available.
// How to make this configurable? We crawl a doc we don't support it goes to
// a matcher where it will just match the first language supported. Tricky.
// Once we are ready look at wikipedia package implementation.
func Languages(supported []language.Tag) []language.Tag {
	return available
}

// Matcher is a language matcher.
// Will need to change if we can figure out language customization (see note above)
var Matcher = language.NewMatcher(available) // globals...ugh!

// available is a slice of supported languages.
// Languages below are taken verbatim from the languages package.
// https://godoc.org/golang.org/x/text/language#Tag
// We can add more languages to this slice.
// Commented means we are unsure of the appropriate Elasticsearch analyzer.
var available = []language.Tag{
	language.English, //  en // The first one is our fallback language
	//language.Afrikaans,            //  af
	//language.Amharic,              //  am
	language.Arabic, //  ar
	//language.ModernStandardArabic, //  ar-001
	//language.Azerbaijani,          //  az
	language.Bulgarian, //  bg
	//language.Bengali,              //  bn
	language.Catalan,              //  ca
	language.Czech,                //  cs
	language.Danish,               //  da
	language.German,               //  de
	language.Greek,                //  el
	language.AmericanEnglish,      //  en-US
	language.BritishEnglish,       //  en-GB
	language.Spanish,              //  es
	language.EuropeanSpanish,      //  es-ES
	language.LatinAmericanSpanish, //  es-419
	//language.Estonian,             //  et
	language.Persian, //  fa
	language.Finnish, //  fi
	//language.Filipino,             //  fil
	language.French,         //  fr
	language.CanadianFrench, //  fr-CA
	//language.Gujarati,             //  gu
	//language.Hebrew,               //  he
	language.Hindi, //  hi
	//language.Croatian,             //  hr
	language.Hungarian,  //  hu
	language.Armenian,   //  hy
	language.Indonesian, //  id
	//language.Icelandic,            //  is
	language.Italian,  //  it
	language.Japanese, //  ja
	//language.Georgian,             //  ka
	//language.Kazakh,               //  kk
	//language.Khmer,                //  km
	//language.Kannada,              //  kn
	language.Korean, //  ko
	//language.Kirghiz,              //  ky
	//language.Lao,                  //  lo
	language.Lithuanian, //  lt
	language.Latvian,    //  lv
	//language.Macedonian,           //  mk
	//language.Malayalam,            //  ml
	//language.Mongolian,            //  mn
	//language.Marathi,              //  mr
	//language.Malay,                //  ms
	//language.Burmese,              //  my
	//language.Nepali,               //  ne
	language.Dutch,     //  nl
	language.Norwegian, //  no
	//language.Punjabi,              //  pa
	//language.Polish,               //  pl
	language.Portuguese,          //  pt
	language.BrazilianPortuguese, //  pt-BR
	language.EuropeanPortuguese,  //  pt-PT
	language.Romanian,            //  ro
	language.Russian,             //  ru
	//language.Sinhala,              //  si
	//language.Slovak,               //  sk
	//language.Slovenian,            //  sl
	//language.Albanian,             //  sq
	//language.Serbian,              //  sr
	//language.SerbianLatin,         //  sr-Latn
	language.Swedish, //  sv
	//language.Swahili,              //  sw
	//language.Tamil,                //  ta
	//language.Telugu,               //  te
	language.Thai,    //  th
	language.Turkish, //  tr
	//language.Ukrainian,            //  uk
	//language.Urdu,                 //  ur
	//language.Uzbek,                //  uz
	language.Vietnamese,         //  vi
	language.Chinese,            //  zh
	language.SimplifiedChinese,  //  zh-Hans
	language.TraditionalChinese, //  zh-Hant
	//language.Zulu,                 //  zu
}
