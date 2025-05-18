package main

import (
	"fmt"
	"net/url"
	"sync"
	"testing"
)

func Test_Next(t *testing.T) {
	pool := &ServerPool{
		backends: []*Backend{
			{
				URL:          &url.URL{Host: "test.com", Path: "/test1"},
				Alive:        true,
				mux:          sync.RWMutex{},
				ReverseProxy: nil,
			},
			{
				URL:          &url.URL{Host: "test.com", Path: "/test2"},
				Alive:        false,
				mux:          sync.RWMutex{},
				ReverseProxy: nil,
			},
			{
				URL:          &url.URL{Host: "test.com", Path: "/test3"},
				Alive:        true,
				mux:          sync.RWMutex{},
				ReverseProxy: nil,
			},
			{
				URL:          &url.URL{Host: "test.com", Path: "/test4"},
				Alive:        true,
				mux:          sync.RWMutex{},
				ReverseProxy: nil,
			},
		},
	}

	for i := 0; i < 10; i++ {
		fmt.Println(pool.GetNextPeer().URL.String())
	}
}
