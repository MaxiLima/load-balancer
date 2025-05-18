package main

import (
	"load-balancer/cmd/internal/balancer"
)

func main() {
	b := balancer.New()
	b.Serve()
}
