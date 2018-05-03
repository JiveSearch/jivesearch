// Package bangs provides functionality to query other websites
package bangs

import (
	"strings"

	"golang.org/x/text/language"
)

// Bangs holds a map of !bangs
type Bangs struct {
	Bangs []Bang
	Suggester
}

// Bang holds a single !bang
type Bang struct {
	Name      string            `json:"name"`
	Triggers  []string          `json:"triggers"`
	Regions   map[string]string `json:"regions"`
	Functions []fn              `json:"-"`
}

// Suggester is a !bangs suggester/autocomplete
type Suggester interface {
	IndexExists() (bool, error)
	DeleteIndex() error
	Setup([]Bang) error
	SuggestResults(term string, size int) (Results, error)
}

// Results are the results of an autocomplete query
type Results struct { // remember top-level arrays = no-no in javascript/json
	Suggestions []Suggestion `json:"suggestions"`
}

// Suggestion is an individual !bang autocomplete suggestion
type Suggestion struct {
	Trigger string `json:"trigger"`
	Name    string `json:"name"`
}

const def = "default"

// Suggest is an autocomplete for !bangs
func (b *Bangs) Suggest(term string, size int) (Results, error) {
	res, err := b.Suggester.SuggestResults(term, size)
	if err != nil {
		return res, err
	}

	// fill in the rest of the suggestion
	for i, s := range res.Suggestions {
		for _, bng := range b.Bangs {
			for _, trigger := range bng.Triggers {
				if trigger == s.Trigger {
					s.Name = bng.Name
					res.Suggestions[i] = s
				}
			}
		}
	}

	return res, err
}

// Detect lets us know if we have a !bang match.
func (b *Bangs) Detect(q string, region language.Region, l language.Tag) (string, bool) {
	fields := strings.Fields(q)

	for i, field := range fields {
		if field == "!" || (!strings.HasPrefix(field, "!") && !strings.HasSuffix(field, "!")) {
			continue
		}

		k := strings.ToLower(strings.Trim(field, "!"))
		for _, bng := range b.Bangs {
			if triggered := trigger(k, bng.Triggers); !triggered {
				continue
			}

			remainder := strings.Join(append(fields[:i], fields[i+1:]...), " ")

			for _, f := range bng.Functions {
				remainder = f(remainder)
			}

			for _, reg := range []string{strings.ToLower(region.String()), def} { // use default region if no region specified
				if u, ok := bng.Regions[reg]; ok {
					u = strings.Replace(u, "{{{term}}}", remainder, -1)
					return strings.Replace(u, "{{{lang}}}", l.String(), -1), true
				}
			}
		}
	}
	return "", false
}

type fn func(string) string

// Returns the canonical version of a Wikipedia title.
// "bob maRLey" -> "Bob_Marley"
// Some regional queries will be incorrect: https://es.wikipedia.org/wiki/De_la_Tierra_a_la_Luna
var wikipediaCanonical = func(q string) string {
	return strings.Replace(strings.Title(strings.ToLower(q)), " ", "_", -1)
}

func trigger(k string, triggers []string) bool {
	for _, trigger := range triggers {
		if k == trigger {
			return true
		}
	}
	return false
}

// New creates a pointer with the default !bangs.
// Use default url unless a region is provided.
// Region: US, Language: French !a ---> Amazon.com
// Region: France, Language: English !a ---> Amazon.fr
// !afr ---> Amazon.fr
// Note: Some !bangs don't respect the language passed in or
// may not support it (eg they may support pt but not pt-BR)
//
// TODO: Allow overrides...perhaps add a method or use a config.
// Note: If we end up using viper for this don't use "SetDefault"
// as overriding one !bang will replace ALL !bangs. Instead, use "Set".
func New() *Bangs {
	// Not sure about the structure here...slice of Bangs makes it easy to add bangs
	// Would like to add autocomplete feature so that people can find !bangs easier.
	b := &Bangs{}
	b.Bangs = []Bang{
		{
			"Amazon", []string{"a", "amazon"},
			map[string]string{
				def:  "https://www.amazon.com/s/ref=nb_sb_noss?url=search-alias%3Daps&field-keywords={{{term}}}",
				"ca": "https://www.amazon.ca/s/ref=nb_sb_noss?url=search-alias%3Daps&field-keywords={{{term}}}",
				"fr": "https://www.amazon.fr/s/ref=nb_sb_noss?url=search-alias%3Daps&field-keywords={{{term}}}",
				"uk": "https://www.amazon.co.uk/s/ref=nb_sb_noss?url=search-alias%3Daps&field-keywords={{{term}}}",
			},
			[]fn{},
		},
		{
			"Bing", []string{"b", "bing"},
			map[string]string{
				def: "https://www.bing.com/search?q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Bing Images", []string{"bi", "bingimages"},
			map[string]string{
				def: "https://www.bing.com/images/search?q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Bitbucket", []string{"bb", "bitbucket"},
			map[string]string{
				def: "https://bitbucket.org/repo/all/?name={{{term}}}",
			},
			[]fn{},
		},
		{
			"CNBC", []string{"cnbc"},
			map[string]string{
				def: "http://search.cnbc.com/main.do?target=all&keywords={{{term}}}",
			},
			[]fn{},
		},
		{
			"GitHub", []string{"gh", "git", "github"},
			map[string]string{
				def: "https://github.com/search?q={{{term}}}&type=Everything&repo=&langOverride=&start_value=1",
			},
			[]fn{},
		},
		{
			"eBay", []string{"e", "ebay"},
			map[string]string{
				def: "https://www.ebay.com/sch/items/?_nkw={{{term}}}&_sacat=&_ex_kw=&_mPrRngCbx=1&_udlo=&_udhi=&_sop=12&_fpos=&_fspt=1&_sadis=&LH_CAds=&rmvSB=true",
			},
			[]fn{},
		},
		{
			"Genius", []string{"genius"},
			map[string]string{
				def: "https://genius.com/search?q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Google", []string{"g", "google"},
			map[string]string{
				def:  "https://encrypted.google.com/search?hl={{{lang}}}&q={{{term}}}",
				"ca": "https://www.google.ca/search?q={{{term}}}",
				"fr": "https://www.google.fr/search?hl={{{lang}}}&q={{{term}}}",
				"ru": "https://www.google.ru/search?hl={{{lang}}}&q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Google France", []string{"gfr", "googlefr"},
			map[string]string{
				def: "https://www.google.fr/search?hl={{{lang}}}&q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Google Images", []string{"gi"},
			map[string]string{
				def: "https://www.google.com/search?q={{{term}}}&source=lnms&tbm=isch",
			},
			[]fn{},
		},
		{
			"Google Russia", []string{"gru", "googleru"},
			map[string]string{
				def: "https://www.google.ru/search?hl={{{lang}}}&q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Google Scholar", []string{"googlescholar"},
			map[string]string{
				def: "https://scholar.google.com/scholar?hl={{{lang}}}&q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Google Translate", []string{"gt", "googletranslate"},
			map[string]string{
				def: "https://translate.google.com/#auto/en/{{{term}}}",
			},
			[]fn{},
		},
		{
			"Hacker News", []string{"hn", "hackernews"},
			map[string]string{
				def: "https://hn.algolia.com/?query={{{term}}}&sort=byPopularity&prefix&page=0&dateRange=all&type=story",
			},
			[]fn{},
		},
		{
			"Hulu", []string{"hulu"},
			map[string]string{
				def: "https://www.hulu.com/search?query={{{term}}}",
			},
			[]fn{},
		},
		{
			"IMDb", []string{"imdb"},
			map[string]string{
				def: "http://www.imdb.com/find?q={{{term}}}&s=all",
			},
			[]fn{},
		},
		{
			"Instagram", []string{"instagram", "ig"},
			map[string]string{
				def: "https://www.instagram.com/explore/tags/{{{term}}}",
			},
			[]fn{},
		},
		{
			"LinkedIn", []string{"linkedin", "li"},
			map[string]string{
				def: "https://www.linkedin.com/search/results/index/?keywords={{{term}}}",
			},
			[]fn{},
		},
		{
			"Movie DB", []string{"moviedb"},
			map[string]string{
				def: "https://www.themoviedb.org/search?query={{{term}}}",
			},
			[]fn{},
		},
		{
			"New York Times", []string{"nyt", "nytimes", "newyorktimes"},
			map[string]string{
				def: "http://query.nytimes.com/search/sitesearch/#/{{{term}}}/",
			},
			[]fn{},
		},
		{
			"Reddit", []string{"reddit"},
			map[string]string{
				def: "https://www.reddit.com/search?q={{{term}}}&restrict_sr=&sort=relevance&t=all",
			},
			[]fn{},
		},
		{
			"Rotten Tomatoes", []string{"rt"},
			map[string]string{
				def: "https://www.rottentomatoes.com/search/?search={{{term}}}",
			},
			[]fn{},
		},
		{
			"SEC", []string{"sec", "edgar"},
			map[string]string{
				def: "https://www.sec.gov/cgi-bin/browse-edgar?CIK={{{term}}}&owner=exclude&action=getcompany",
			},
			[]fn{},
		},
		{
			"Stack Overflow", []string{"so", "stackoverflow"},
			map[string]string{
				def: "https://stackoverflow.com/search?q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Steam", []string{"steam"},
			map[string]string{
				def: "http://store.steampowered.com/search/?term={{{term}}}",
			},
			[]fn{},
		},
		{
			"TV Database", []string{"tvdb"},
			map[string]string{
				def: "https://www.thetvdb.com/?string={{{term}}}&tab=listseries&function=Search",
			},
			[]fn{},
		},
		{
			"Twitter", []string{"t", "tw", "twitter"},
			map[string]string{
				def: "https://twitter.com/search?q={{{term}}}",
			},
			[]fn{},
		},
		{
			"Wall Street Journal", []string{"wsj", "wallstreetjournal"},
			map[string]string{
				def: "https://www.wsj.com/search/term.html?KEYWORDS={{{term}}}&isAdvanced=true&daysback=90d&andor=AND&sort=date-desc&source=wsjarticle,wsjblogs,wsjvideo,sitesearch,press,newswire",
			},
			[]fn{},
		},
		{
			// I think these need to be mapped to languages, not regions...
			"Wikipedia", []string{"w", "wikipedia"},
			map[string]string{
				def:  "https://en.wikipedia.org/wiki/{{{term}}}",
				"es": "https://es.wikipedia.org/wiki/{{{term}}}",
				"de": "https://de.wikipedia.org/wiki/{{{term}}}",
				"fr": "https://fr.wikipedia.org/wiki/{{{term}}}",
			},
			[]fn{wikipediaCanonical},
		},
		{
			"Yahoo", []string{"y", "yahoo"},
			map[string]string{
				def: "https://search.yahoo.com/search?p={{{term}}}",
			},
			[]fn{},
		},
		{
			"Yahoo Finance", []string{"yf", "yahoofinance"},
			map[string]string{
				def: "https://finance.yahoo.com/quote/{{{term}}}",
			},
			[]fn{},
		},
		{
			"Yahoo Finance Charts", []string{"yfc"},
			map[string]string{
				def: "https://finance.yahoo.com/chart/{{{term}}}",
			},
			[]fn{},
		},
		{
			"Yahoo Finance Profile", []string{"yfp"},
			map[string]string{
				def: "https://finance.yahoo.com/quote/{{{term}}}/profile",
			},
			[]fn{},
		},
		{
			"Yahoo Finance Stats", []string{"yfs"},
			map[string]string{
				def: "https://finance.yahoo.com/quote/{{{term}}}/key-statistics",
			},
			[]fn{},
		},
		{
			"Youtube", []string{"yt", "youtube"},
			map[string]string{
				def: "https://www.youtube.com/results?search_query={{{term}}}",
			},
			[]fn{},
		},
	}

	return b
}
