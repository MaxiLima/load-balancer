package instance

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"

	"load-balancer/cmd/internal/platform/web"
)

type Backend struct {
	port         int
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

func New(port int, errCallback http.HandlerFunc) *Backend {
	u, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))

	rp := httputil.NewSingleHostReverseProxy(u)

	rp.Director = func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host
	}

	back := &Backend{
		port:         port,
		URL:          u,
		Alive:        true,
		mux:          sync.RWMutex{},
		ReverseProxy: rp,
	}

	back.ReverseProxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		log.Printf("%s: %s\n", u.Host, err.Error())
		retries := web.GetRetryFromContext(request)
		if retries < 3 {
			select {
			case <-time.After(10 * time.Millisecond):
				ctx := context.WithValue(request.Context(), web.Retry, retries+1)
				rp.ServeHTTP(writer, request.WithContext(ctx))
			}
		}

		back.SetAlive(false)

		attempts := web.GetAttemptsFromContext(request)
		log.Printf("%s(%s) attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
		ctx := context.WithValue(request.Context(), web.Attempts, attempts+1)
		errCallback(writer, request.WithContext(ctx))
	}

	return back
}

func (b *Backend) RunProxy() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode("hello from port " + strconv.Itoa(b.port))
	})

	log.Printf("Starting backend server on port %d\n", b.port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(b.port), mux))
}

func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	defer b.mux.Unlock()
	b.Alive = alive
}

func (b *Backend) Ping() bool {
	// from time to time we simulate a faulty instance so that we can see how the balancer handles it
	// we have a 10% chance of getting 5 out of the interval [0, 9).
	if rand.Intn(9) == 5 {
		return false
	}

	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", b.URL.Host, timeout)
	if err != nil {
		log.Println("Host unreachable: ", err.Error())
		return false
	}

	_ = conn.Close()
	return true
}
