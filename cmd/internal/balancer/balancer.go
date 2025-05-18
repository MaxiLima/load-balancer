package balancer

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"load-balancer/cmd/internal/instance"
	"load-balancer/cmd/internal/platform/web"
	"load-balancer/cmd/internal/port"
)

type Balancer struct {
	porter *port.Service
	pool   *ServerPool
}

func New(instances int, porter *port.Service) *Balancer {
	pool := &ServerPool{backends: make([]*instance.Backend, 0)}
	balancer := &Balancer{porter: porter, pool: pool}

	for i := 0; i < instances; i++ {
		instancePort, err := porter.GetNext()
		if err != nil {
			log.Fatal(err)
		}

		backend := instance.New(instancePort, balancer.lb)

		balancer.pool.backends = append(balancer.pool.backends, backend)
		go backend.RunProxy()

	}

	return balancer
}

func (b *Balancer) Serve() {
	router := chi.NewRouter()

	router.Handle("/hello", http.HandlerFunc(b.lb))

	go b.healthCheck()

	log.Printf("Server running at http://localhost:%d\n", b.porter.GetBasePort())
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", b.porter.GetBasePort()), router))
}

func (b *Balancer) lb(w http.ResponseWriter, r *http.Request) {
	attempts := web.GetAttemptsFromContext(r)
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
