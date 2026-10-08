package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var (
	base     = flag.String("base", "http://localhost:8080", "shortener base URL")
	data     = flag.String("data", "testdata/urls.txt", "file with input URLs, one per line")
	duration = flag.Duration("duration", 20*time.Second, "load duration")
	workers  = flag.Int("workers", 50, "concurrent load workers")
)

type stats struct {
	mu     sync.Mutex
	lat    []time.Duration
	status map[int]int
	err    int64
}

func (s *stats) add(lat time.Duration, code int) {
	s.mu.Lock()
	s.lat = append(s.lat, lat)
	s.status[code]++
	s.mu.Unlock()
}

func (s *stats) addErr() { atomic.AddInt64(&s.err, 1) }

func readURLs(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open data:", err)
		os.Exit(1)
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
		fmt.Fprintln(os.Stderr, "read data:", err)
		os.Exit(1)
	}
	return urls
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

	var seq int64
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

				n := atomic.AddInt64(&seq, 1)
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
	sort.Slice(st.lat, func(i, j int) bool { return st.lat[i] < st.lat[j] })
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
	errcount := int(atomic.LoadInt64(&st.err))
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

	urls := readURLs(*data)
	do := client(*workers)

	resp, err := do.Get(*base + "/ping")
	if err != nil {
		fmt.Fprintln(os.Stderr, "shortener unreachable:", err)
		os.Exit(1)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	fmt.Printf("seeding %d URLs ...\n", len(urls))
	seedStart := time.Now()
	codes := seed(do, urls)
	fmt.Printf("seeded %d short links in %s\n", len(codes), time.Since(seedStart))
	if len(codes) == 0 {
		fmt.Fprintln(os.Stderr, "no short links produced")
		os.Exit(1)
	}
	fmt.Printf("sample short URL: %s\n", codes[0])

	run(do, urls, codes)
}
