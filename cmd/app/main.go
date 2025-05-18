package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type Backend struct {
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

type ServerPool struct {
	backends []*Backend
	current  uint64
}

const (
	Attempts int = iota
	Retry
)

func GetRetryFromContext(r *http.Request) int {
	if retry, ok := r.Context().Value(Retry).(int); ok {
		return retry
	}
	return 0
}

func GetAttemptsFromContext(r *http.Request) int {
	if attempts, ok := r.Context().Value(Attempts).(int); ok {
		return attempts
	}

	return 0
}

type Balancer struct {
	pool *ServerPool
}

func main() {
	pool := &ServerPool{backends: make([]*Backend, 0)}
	balancer := &Balancer{pool}

	for i := 1; i < 5; i++ {
		port := 8080 + i
		u, _ := url.Parse(fmt.Sprintf("http://localhost:%d/hello", port))
		go startProxyBackend(port)

		rp := httputil.NewSingleHostReverseProxy(u)

		rp.Director = func(req *http.Request) {
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
			req.Host = u.Host
		}

		rp.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
			log.Printf("%s: %s\n", u.Host, err.Error())
			retries := GetRetryFromContext(request)
			if retries < 3 {
				select {
				case <-time.After(10 * time.Millisecond):
					ctx := context.WithValue(request.Context(), Retry, retries+1)
					rp.ServeHTTP(writer, request.WithContext(ctx))
				}
			}

			pool.MarkBackendStatus(u, false)

			attempts := GetAttemptsFromContext(request)
			log.Printf("%s(%s) attempting retry %d\n", request.RemoteAddr, request.URL.Path, attempts)
			ctx := context.WithValue(request.Context(), Attempts, attempts+1)
			balancer.lb(writer, request.WithContext(ctx))
		}

		pool.backends = append(pool.backends, &Backend{
			URL:          u,
			Alive:        true,
			mux:          sync.RWMutex{},
			ReverseProxy: rp,
		})
	}

	go balancer.healthCheck()

	router := chi.NewRouter()

	router.Handle("/hello", http.HandlerFunc(balancer.lb))

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

func startProxyBackend(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode("hello from port " + strconv.Itoa(port))
	})

	log.Printf("Starting backend server on port %d\n", port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), mux))
}

func (b *Balancer) lb(w http.ResponseWriter, r *http.Request) {
	attempts := GetAttemptsFromContext(r)
	if attempts > 3 {
		log.Printf("%s(%s) Max attempts reached, terminating\n", r.RemoteAddr, r.URL.Path)
		http.Error(w, "Service not available", http.StatusServiceUnavailable)
		return
	}

	peer := b.pool.GetNextPeer()
	if peer != nil {
		log.Printf("sending request to %s\n", peer.URL.String())
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}

	http.Error(w, "service unavailable", http.StatusServiceUnavailable)
}

func isBackendAlive(u *url.URL) bool {
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", u.Host, timeout)
	if err != nil {
		log.Println("Host unreachable: ", err.Error())
		return false
	}

	_ = conn.Close()
	return true
}

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		alive := isBackendAlive(b.URL)
		b.SetAlive(alive)
		log.Printf("(%s) backend ping returned %v\n", b.URL.String(), alive)
	}
}

func (s *ServerPool) MarkBackendStatus(u *url.URL, alive bool) {
	for _, b := range s.backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			log.Printf("(%s) backend status set to %v\n", b.URL.String(), alive)
			return
		}
	}
}

func (b *Balancer) healthCheck() {
	ticker := time.NewTicker(20 * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Println("Performing health check")
			b.pool.HealthCheck()
			log.Println("Health check completed")
		}
	}
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) GetNextPeer() *Backend {
	next := s.NextIndex()
	l := len(s.backends) + next
	for i := next; i < l; i++ {
		idx := i % len(s.backends)

		if s.backends[idx].IsAlive() {
			if i != next {
				atomic.StoreUint64(&s.current, uint64(idx))
			}

			return s.backends[idx]
		}
	}
	return nil
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
