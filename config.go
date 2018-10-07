package traffic

import (
	"log"
	"os"
)

// Config Application config definition
type Config struct {
	Input   InputConfig   `yaml:"input"`
	Rollbar RollbarConfig `yaml:"rollbar"`
	DSN     string        `yaml:"dsn"`
}

// LoadConfig LoadConfig
func LoadConfig() Config {

	config := Config{
		Input: InputConfig{
			Address: "amqp://guest:guest@" + os.Getenv("TRAFFIC_INPUT_HOST") + ":" + os.Getenv("TRAFFIC_INPUT_PORT") + "/",
			Queue:   os.Getenv("TRAFFIC_INPUT_QUEUE"),
		},
		Rollbar: RollbarConfig{
			Token:       os.Getenv("TRAFFIC_ROLLBAR_TOKEN"),
			Environment: os.Getenv("TRAFFIC_ROLLBAR_ENVIRONMENT"),
			Period:      os.Getenv("TRAFFIC_ROLLBAR_PERIOD"),
		},
		DSN: os.Getenv("TRAFFIC_MYSQL_USERNAME") + ":" + os.Getenv("TRAFFIC_MYSQL_PASSWORD") +
			"@tcp(" + os.Getenv("TRAFFIC_MYSQL_HOST") + ":" + os.Getenv("TRAFFIC_MYSQL_PORT") + ")/" +
			os.Getenv("TRAFFIC_MYSQL_DBNAME") + "?charset=utf8mb4&parseTime=true&loc=UTC",
	}

	return config
}

// ValidateConfig ValidateConfig
func ValidateConfig(config Config) {
	if config.Input.Address == "" {
		log.Fatalln("Address not provided")
	}

	if config.Input.Queue == "" {
		log.Fatalln("Queue not provided")
	}
}
