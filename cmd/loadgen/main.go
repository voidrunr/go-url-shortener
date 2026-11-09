package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	base     = flag.String("base", "http://localhost:8080", "shortener base URL")
	data     = flag.String("data", "testdata/urls.txt", "file with input URLs, one per line")
	duration = flag.Duration("duration", 20*time.Second, "load duration")
	workers  = flag.Int("workers", 50, "concurrent load workers")
)

type stats struct {
	status map[int]int
	lat    []time.Duration
	err    atomic.Int64
	mu     sync.Mutex
}

func (s *stats) add(lat time.Duration, code int) {
	s.mu.Lock()
	s.lat = append(s.lat, lat)
	s.status[code]++
	s.mu.Unlock()
}

func (s *stats) addErr() { s.err.Add(1) }

func readURLs(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open data: %w", err)
	}
	defer f.Close()

	var urls []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			urls = append(urls, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read data: %w", err)
	}
	return urls, nil
}

func client(n int) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: n + 1,
			MaxIdleConns:        n + 1,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func seed(do *http.Client, urls []string) []string {
	codes := make([]string, 0, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 32)

	for _, u := range urls {
		wg.Add(1)
		sem <- struct{}{}
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()

			resp, err := do.Post(*base+"/", "text/plain", bytes.NewBufferString(u))
			if err != nil {
				fmt.Fprintln(os.Stderr, "seed error:", err)
				return
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusCreated {
				mu.Lock()
				codes = append(codes, string(body))
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()
	return codes
}

func pct(lat []time.Duration, p float64) time.Duration {
	if len(lat) == 0 {
		return 0
	}
	idx := int(p*float64(len(lat)) + 0.5)
	if idx >= len(lat) {
		idx = len(lat) - 1
	}
	return lat[idx]
}

func run(do *http.Client, urls, codes []string) {
	start := time.Now()
	done := make(chan struct{})
	deadline := time.NewTimer(*duration)
	defer deadline.Stop()

	var seq atomic.Int64
	var st stats
	st.status = map[int]int{}

	for w := 0; w < *workers; w++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
				}

				n := seq.Add(1)
				beg := time.Now()
				var resp *http.Response
				var err error
				if n%4 == 0 {
					u := urls[int(n)%len(urls)] + fmt.Sprintf("?lo=%d", n)
					resp, err = do.Post(*base+"/", "text/plain", bytes.NewBufferString(u))
				} else {
					resp, err = do.Get(codes[int(n)%len(codes)])
				}
				if err != nil {
					st.addErr()
					continue
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				st.add(time.Since(beg), resp.StatusCode)
			}
		}()
	}

	<-deadline.C
	close(done)
	elapsed := time.Since(start)

	st.mu.Lock()
	slices.Sort(st.lat)
	total := len(st.lat)
	var codes2xx, codes3xx int
	for code, count := range st.status {
		switch {
		case code >= 200 && code < 300:
			codes2xx += count
		case code >= 300 && code < 400:
			codes3xx += count
		}
	}
	errcount := int(st.err.Load())
	st.mu.Unlock()

	fmt.Println("Summary:")
	fmt.Printf("  Total:   %.1fs\n", elapsed.Seconds())
	fmt.Printf("  Requests: %d   [rps: %.1f]\n", total, float64(total)/elapsed.Seconds())
	fmt.Printf("  Latency: p50=%s  p95=%s  p99=%s\n",
		pct(st.lat, 0.5), pct(st.lat, 0.95), pct(st.lat, 0.99))
	fmt.Printf("  Status 2xx=%d 3xx=%d others=%v\n", codes2xx, codes3xx, st.status)
	fmt.Printf("  Errors (network): %d\n", errcount)
}

func main() {
	flag.Parse()

	urls, err := readURLs(*data)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read input data")
	}
	do := client(*workers)

	resp, err := do.Get(*base + "/ping")
	if err != nil {
		log.Fatal().Err(err).Msg("shortener unreachable")
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	fmt.Printf("seeding %d URLs ...\n", len(urls))
	seedStart := time.Now()
	codes := seed(do, urls)
	fmt.Printf("seeded %d short links in %s\n", len(codes), time.Since(seedStart))
	if len(codes) == 0 {
		log.Fatal().Msg("no short links produced")
	}
	fmt.Printf("sample short URL: %s\n", codes[0])

	run(do, urls, codes)
}
