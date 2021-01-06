package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/autowp/traffic"
)

func main() {

	config := traffic.LoadConfig()

	traffic.ValidateConfig(config)

	t, err := traffic.NewService(config)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
		return
	}

	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "migrate":
		err = t.Migrate()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}
		t.Close()
		os.Exit(0)
		return
	case "scheduler-hourly":
		err = t.SchedulerHourly()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}
		t.Close()
		os.Exit(0)
		return
	case "scheduler-minutely":
		err = t.SchedulerMinutely()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}
		t.Close()
		os.Exit(0)
		return
	case "serve":
		err = t.Serve()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}

		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		for sig := range c {
			log.Printf("captured %v, stopping and exiting.", sig)

			t.Close()
			os.Exit(1)
		}
		return
	case "listen-amqp":
		quit := make(chan bool)
		err = t.ListenAMQP(quit)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
			return
		}

		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		for sig := range c {
			log.Printf("captured %v, stopping and exiting.", sig)

			quit <- true
			close(quit)
			t.Close()
			os.Exit(1)
		}
		return
	}

	t.Close()
	os.Exit(0)
}
