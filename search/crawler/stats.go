package crawler

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	humanize "github.com/dustin/go-humanize"
)

// Stats keeps track of time elapsed & status codes
type Stats struct {
	sync.Mutex
	Start       time.Time
	elapsed     time.Duration
	StatusCodes map[int]int64
}

// Update our stats from a document's results
func (s *Stats) Update(code int) {
	s.Lock()
	s.StatusCodes[code]++
	s.Unlock()
}

// Elapsed will set the total time the crawler has been running
func (s *Stats) Elapsed() *Stats {
	s.elapsed = time.Since(s.Start)
	return s
}

// Print our stats into human-readable
func (s *Stats) String() string {
	var total int64
	codes := make(map[string]int64)
	for k, v := range s.StatusCodes {
		code := strconv.Itoa(k)[:1] + "xx"
		if k == -1 {
			code = "Not Crawled"
		}
		total += v
		codes[code] += v
	}

	if total == 0 {
		return ""
	}

	// calculate our crawl rate
	nano := float64(s.elapsed) // time.Duration is in nanoseconds
	micro := nano / 1000
	milli := micro / 1000
	second := milli / 1000
	rps := total / int64(second)

	stats := fmt.Sprintf("[stats] Crawled: %v Elapsed: %v\n", total, s.elapsed)
	stats += fmt.Sprintf("[stats] Rate: %v per second, %v per minute, %v per hour, %v per day\n",
		humanize.Comma(rps), humanize.Comma(rps*60), humanize.Comma(rps*60*60), humanize.Comma(rps*60*60*24))

	stats += fmt.Sprint("[stats] Status codes: ")

	keys := []string{}
	for k := range codes {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		stats += fmt.Sprintf("%v (%v%%)  ", k, strconv.Itoa(int(100*codes[k]/total)))
	}

	stats += fmt.Sprintf("\n")
	return stats
}
