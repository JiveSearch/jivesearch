Jive Search (name subject to change) is a completely open source search engine that respects your privacy. 

## Table of Contents
- [Quick start](#quick-start)
- [Installation](#installation)
- [Status](#status)
- [Roadmap](#roadmap)
- [Bugs and feature requests](#bugs-and-feature-requests)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [Community](#community)
- [Creators](#creators)
- [Copyright and license](#copyright-and-license)

## Quick Start
1. Go to Jive Search's [homepage](https://www.jivesearch.com).
2. Start searching.
3. Upvote or downvote the pages!

## Installation

1. Download Go [here](https://golang.org/dl/).
2. Set your GOPATH, steps [here](https://github.com/golang/go/wiki/SettingGOPATH)
3. Install Jive Search

```
go get -u github.com/jivesearch/jivesearch
```

##### Crawler
Requires Elasticsearch and Redis.
```
cd $GOPATH/src/github.com/jivesearch/jivesearch/search/crawler && go run ./cmd/crawler.go --workers=75 --time=5m --debug=true
```

##### Frontend
Requires Elasticsearch and PostgreSQL.
```
cd $GOPATH/src/github.com/jivesearch/jivesearch/frontend && go run ./cmd/frontend.go --debug=true
```

##### Wikipedia Dump File
Requires PostgreSQL.
```
cd $GOPATH/src/github.com/jivesearch/jivesearch/instant/wikipedia/cmd/dumper && go run dumper.go --workers=3 --dir=/path/to/wiki/files --text=true --data=true --truncate=400
```
## Status
[![Build Status](https://travis-ci.org/jivesearch/jivesearch.svg?branch=master)](https://travis-ci.org/jivesearch/jivesearch)

## **Roadmap** 
### Our goal is to create a search engine that respects your privacy AND delivers great search results, instant answers, maps, image search, news, and more. 

Marked items indicate progress has been made in that category. There is much more to do in each area. Suggestions are welcomed!

- [x] Privacy
- [x] !Bangs
- [x] Core Search Results & Distributed Crawler
    - [x] Language & Region
    - [ ] Advanced Search (exact phrase, dogs OR cats,  -cats, site/domain search, etc.)
    - [ ] Filetype
    - [ ] SafeSearch        
    - [ ] Time Search (past year/month/day/hour, etc.
- [x] Autocomplete
- [ ] Phrase Suggester (a.k.a. "Did You Mean?")
- [x] Instant Answers
    - [x] Birthstone, camelcase, characters, coin toss, frequency, POTUS, prime, random, reverse, stats, user agent, etc. 
    - [x] Discography/Music albums
    - [x] JavaScript-based answers
        - [x] Basic calculator
            - [ ] Mortgage, financial and other calculators
        - [x] CSS/JavaScript/JSON/etc minifier and prettifier
        - [x] Converters (meters to feet, mb to gb, etc...)
    - [x] Package Tracking (UPS, FedEx, USPS, etc...)
    - [x] Stack Overflow
    - [x] Stock Quotes & Charts    
    - [x] Weather
    - [x] Wikipedia summary
    - [x] Wikidata answers (how tall is, birthday, etc.)
    - [x] Wikiquote
    - [x] Wiktionary    
    - [ ] Many more instant answers (including from 3rd party APIs)
    - [ ] Translate trigger words and answers to other languages?
- [ ] Maps
- [ ] Image Search
- [ ] Video Search
- [ ] Flight Info & Status
- [ ] News
- [ ] Shopping
- [ ] Custom Themes

## Documentation
Jivesearch's documentation is hosted on GoDoc Page at <https://godoc.org/github.com/jivesearch/jivesearch>.

## Contributing
Want to contribute? Great! 

Search for existing and closed issues. If your problem or idea is not addressed yet, [please open a new issue](https://github.com/jivesearch/jivesearch/issues/new).

## Creators

**Brentadamson**
- <https://github.com/brentadamson>

## Copyright and License
Code and documentation copyright 2018 the [Jivesearch Authors](https://github.com/jivesearch/jivesearch/pulse). Code and docs released under the [Apache License](https://github.com/jivesearch/jivesearch/blob/master/LICENSE).




