package traffic

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"

	yaml "gopkg.in/yaml.v2"
)

// Config Application config definition
type Config struct {
	Input   InputConfig   `yaml:"input"`
	Rollbar RollbarConfig `yaml:"rollbar"`
	DSN     string        `yaml:"dsn"`
}

// LoadConfig LoadConfig
func LoadConfig() Config {
	var configFile string
	flag.StringVar(&configFile, "config", "config.yml", "path to config file")

	flag.Parse()

	fmt.Printf("Run with config `%s`\n", configFile)

	content, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Failed to read config file: %v", err)
	}

	config := Config{}

	err = yaml.Unmarshal(content, &config)
	if err != nil {
		log.Fatalf("Failed to parse config: %v", err)
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
