package traffic

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	"github.com/streadway/amqp"
)

// InputConfig config part
type InputConfig struct {
	Address string `yaml:"address"`
	Queue   string `yaml:"queue"`
}

// Input AMQP  wrapper
type Input struct {
	config       InputConfig
	conn         *amqp.Connection
	ch           *amqp.Channel
	inQ          amqp.Queue
	handler      func(InputMessage)
	errorHandler func(error)
	quit         chan bool
}

// InputMessage InputMessage
type InputMessage struct {
	IP        net.IP    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
}

// NewInput Constructor
func NewInput(config InputConfig, handler func(InputMessage), errorHandler func(error)) *Input {

	return &Input{
		config:       config,
		handler:      handler,
		errorHandler: errorHandler,
		quit:         make(chan bool),
	}
}

func (input *Input) connect() error {

	if input.conn != nil {
		return nil // already connected
	}

	conn, err := amqp.Dial(input.config.Address)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		return err
	}

	inQ, err := ch.QueueDeclare(
		input.config.Queue, // name
		false,              // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // arguments
	)
	if err != nil {
		return err
	}

	input.conn = conn
	input.ch = ch
	input.inQ = inQ

	return nil
}

// Listen for incoming messages
func (input *Input) Listen() error {
	err := input.connect()
	if err != nil {
		return err
	}

	msgs, err := input.ch.Consume(
		input.inQ.Name, // queue
		"",             // consumer
		true,           // auto-ack
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		return err
	}

	quit := false
	for !quit {
		select {
		case d := <-msgs:
			if d.ContentType != "application/json" {
				input.errorHandler(fmt.Errorf("unexpected mime `%s`", d.ContentType))
				continue
			}

			var message InputMessage
			err := json.Unmarshal(d.Body, &message)
			if err != nil {
				input.errorHandler(fmt.Errorf("failed to parse json `%v`: %s", err, d.Body))
				continue
			}

			input.handler(message)
		case <-input.quit:
			quit = true
		}
	}

	return nil
}

// Close all connections
func (input *Input) Close() {
	input.quit <- true
	close(input.quit)
	input.ch.Close()
	input.conn.Close()
}
