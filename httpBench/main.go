package main

import (
	"flag"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/valyala/fasthttp"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	var GOROUTINES = flag.Int("goroutines", 1000, "concurrency level (number of goroutines to make requests)")
	var REQUESTS = flag.Int("requests", 300, "number of requests per goroutine")
	var REQUEST = flag.String("url", "http://127.0.0.1:2081/hello", "request to perform")
	var CONNECTIONS = flag.Int("connections", 1, "number of TCP connections")
	flag.Parse()

	requestURL, err := url.Parse(*REQUEST)
	if err != nil {
		log.Fatalf("fail to parse request: %v", err)
	}
	log.Printf("stress %v with %v*%v (%v) requests", requestURL.Host, *GOROUTINES, *REQUESTS, *GOROUTINES**REQUESTS)

	client := &fasthttp.PipelineClient{Addr: requestURL.Host, MaxConns: *CONNECTIONS, MaxPendingRequests: *GOROUTINES}

	start := time.Now()
	var ops uint64
	var wg sync.WaitGroup
	for i := 0; i < *GOROUTINES; i++ {
		wg.Add(1)
		go func() {
			for j := 0; j < *REQUESTS; j++ {
				req := fasthttp.AcquireRequest()
				req.SetRequestURI(*REQUEST)
				resp := fasthttp.AcquireResponse()
				err := client.Do(req, resp)
				if err != nil {
					log.Printf("error: %v", err)
				} else {
					atomic.AddUint64(&ops, 1)
					//log.Printf("response: %v", string(resp.Body()))
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	stop := time.Since(start)
	log.Printf("all %v(actually %v) done in %s", *GOROUTINES**REQUESTS, ops, stop)
	log.Printf("estimated %v rps", int64(float64(ops)/stop.Seconds()))
}
