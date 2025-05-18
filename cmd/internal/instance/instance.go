package instance

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"sync"
	"time"
)

type Backend struct {
	port         int
	URL          *url.URL
	Alive        bool
	mux          sync.RWMutex
	ReverseProxy *httputil.ReverseProxy
}

func New(port int) *Backend {
	u, _ := url.Parse(fmt.Sprintf("http://localhost:%d", port))

	rp := httputil.NewSingleHostReverseProxy(u)

	rp.Director = func(req *http.Request) {
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.Host = u.Host
	}

	return &Backend{
		URL:          u,
		Alive:        true,
		mux:          sync.RWMutex{},
		ReverseProxy: rp,
	}
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
	timeout := 2 * time.Second
	conn, err := net.DialTimeout("tcp", b.URL.Host, timeout)
	if err != nil {
		log.Println("Host unreachable: ", err.Error())
		return false
	}

	_ = conn.Close()
	return true
}
