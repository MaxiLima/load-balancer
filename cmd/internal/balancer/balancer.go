package balancer

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"load-balancer/cmd/internal/instance"
)

type Balancer struct {
	pool *ServerPool
}

const (
	Attempts int = iota
	Retry
)

func New() *Balancer {
	pool := &ServerPool{backends: make([]*instance.Backend, 0)}

	for i := 1; i < 5; i++ {
		port := 8080 + i
		backend := instance.New(port)

		/*rp.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
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
		}*/

		pool.backends = append(pool.backends, backend)

		go backend.RunProxy()

	}

	return &Balancer{}
}

func (b *Balancer) Serve() {
	router := chi.NewRouter()

	router.Handle("/hello", http.HandlerFunc(b.lb))

	go b.healthCheck()

	log.Println("Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
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
