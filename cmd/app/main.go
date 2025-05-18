package main

import (
	"load-balancer/cmd/internal/balancer"
	"load-balancer/cmd/internal/port"
)

func main() {
	porter := port.New(10)
	b := balancer.New(porter)
	b.Serve()
}
