package balancer

import (
	"log"
	"net/url"
	"sync/atomic"

	"load-balancer/cmd/internal/instance"
)

type ServerPool struct {
	backends []*instance.Backend
	current  uint64
}

func (s *ServerPool) NextIndex() int {
	return int(atomic.AddUint64(&s.current, uint64(1)) % uint64(len(s.backends)))
}

func (s *ServerPool) GetNextPeer() *instance.Backend {
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

func (s *ServerPool) HealthCheck() {
	for _, b := range s.backends {
		alive := b.Ping()
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
