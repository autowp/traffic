package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/autowp/traffic"
)

func main() {

	config := traffic.LoadConfig()

	traffic.ValidateConfig(config)

	t := traffic.NewService(config)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	for sig := range c {
		log.Printf("captured %v, stopping and exiting.", sig)

		t.Close()
		os.Exit(1)
	}

	t.Close()
	os.Exit(1)
}
