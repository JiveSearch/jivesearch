package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

func TestSetDefaults(t *testing.T) {
	tme := 5 * time.Minute
	cfg := &provider{
		m: map[string]interface{}{},
	}

	SetDefaults(cfg)

	values := []struct {
		key   string
		value interface{}
	}{
		{"hmac.secret", ""},

		// Brand
		{"brand.name", "Jive Search"},
		{"brand.tagline", "The little search engine that could."},
		{"brand.logo", ""},
		{"brand.small_logo", ""},

		// Elasticsearch
		{"elasticsearch.url", "http://127.0.0.1:9200"},
		{"elasticsearch.search.index", "test-search"},
		{"elasticsearch.search.type", "document"},
		{"elasticsearch.bangs.index", "test-bangs"},
		{"elasticsearch.bangs.type", "bang"},
		{"elasticsearch.image.index", "test-images"},
		{"elasticsearch.image.type", "image"},
		{"elasticsearch.query.index", "test-queries"},
		{"elasticsearch.query.type", "query"},
		{"elasticsearch.robots.index", "test-robots"},
		{"elasticsearch.robots.type", "robots"},
		{"elasticsearch.votes.index", "test-votes"},
		{"elasticsearch.votes.type", "vote"},

		// PostgreSQL
		{"postgresql.host", "localhost"},
		{"postgresql.user", "postgres"},
		{"postgresql.password", "password"},
		{"postgresql.database", "jivesearch"},
		{"postgresql.votes.table", "votes"},

		// Redis
		{"redis.host", ""},
		{"redis.port", 6379},

		// crawler defaults
		{"crawler.useragent.full", "https://github.com/jivesearch/jivesearch"},
		{"crawler.useragent.short", "jivesearchbot"},
		{"crawler.time", tme.String()},
		{"crawler.since", 30 * 24 * time.Hour},
		{"crawler.seeds", []string{
			"https://moz.com/top500/domains",
			"https://domainpunch.com/tlds/topm.php",
			"https://www.wikipedia.org/"},
		},
		{"crawler.workers", 100},
		{"crawler.max.bytes", 1024000},
		{"crawler.timeout", 25 * time.Second},
		{"crawler.max.queue.links", 100000},
		{"crawler.max.links", 100},
		{"crawler.max.domain.links", 10000},
		{"crawler.truncate.title", 100},
		{"crawler.truncate.keywords", 25},
		{"crawler.truncate.description", 250},

		{"useragent", "https://github.com/jivesearch/jivesearch"},

		// stackoverflow API settings
		{"stackoverflow.key", "app key"},

		// FedEx package tracking API settings
		{"fedex.account", "account"},
		{"fedex.password", "password"},
		{"fedex.key", "key"},
		{"fedex.meter", "meter"},

		// MaxMind geolocation DB
		{"maxmind.database", "/usr/share/GeoIP/GeoLite2-City.mmdb"},

		// UPS package tracking API settings
		{"ups.user", "user"},
		{"ups.password", "password"},
		{"ups.key", "key"},

		// USPS package tracking API settings
		{"usps.user", "user"},
		{"usps.password", "password"},

		// OpenWeatherMap API settings
		{"openweathermap.key", "key"},

		// wikipedia settings
		{"wikipedia.truncate", 250},
	}

	for _, v := range values {
		got := cfg.Get(v.key)
		if !reflect.DeepEqual(got, v.value) {
			t.Fatalf("key %q; got %+v; want %+v", v.key, got, v.value)
		}
	}
}

type provider struct {
	m map[string]interface{}
}

func (p *provider) SetDefault(key string, value interface{}) {
	p.m[key] = value
}

func (p *provider) SetTypeByDefaultValue(bool) {}

func (p *provider) BindPFlag(key string, flg *pflag.Flag) error {
	return nil
}
func (p *provider) Get(key string) interface{} {
	return p.m[key]
}
func (p *provider) GetString(key string) string { return "" }
func (p *provider) GetInt(key string) int       { return 0 }
func (p *provider) GetStringSlice(key string) []string {
	return p.m[key].([]string)
}
