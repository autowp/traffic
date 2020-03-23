package traffic

import (
	"log"
	"os"

	"github.com/autowp/traffic/util"
)

// HTTPConfig HTTPConfig
type HTTPConfig struct {
	Listen string `yaml:"listen"`
}

// MigrationsConfig MigrationsConfig
type MigrationsConfig struct {
	DSN string `yaml:"dsn"`
	Dir string `yaml:"dir"`
}

// Config Application config definition
type Config struct {
	RabbitMQ        string            `yaml:"rabbitmq"`
	MonitoringQueue string            `yaml:"monitoring_queue"`
	HotlinkQueue    string            `yaml:"hotlink_queue"`
	Sentry          util.SentryConfig `yaml:"sentry"`
	DSN             string            `yaml:"dsn"`
	Migrations      MigrationsConfig  `yaml:"migrations"`
	HTTP            HTTPConfig        `yaml:"http"`
}

// LoadConfig LoadConfig
func LoadConfig() Config {

	config := Config{
		RabbitMQ:        "amqp://guest:guest@" + os.Getenv("TRAFFIC_RABBITMQ_HOST") + ":" + os.Getenv("TRAFFIC_RABBITMQ_PORT") + "/",
		MonitoringQueue: os.Getenv("TRAFFIC_MONITORING_QUEUE"),
		HotlinkQueue:    os.Getenv("TRAFFIC_HOTLINK_QUEUE"),
		Sentry: util.SentryConfig{
			DSN:         os.Getenv("TRAFFIC_SENTRY_DSN"),
			Environment: os.Getenv("TRAFFIC_SENTRY_ENVIRONMENT"),
		},
		DSN: os.Getenv("TRAFFIC_MYSQL_DSN"),
		HTTP: HTTPConfig{
			Listen: os.Getenv("TRAFFIC_HTTP_LISTEN"),
		},
		Migrations: MigrationsConfig{
			DSN: os.Getenv("TRAFFIC_MIGRATIONS_DSN"),
			Dir: os.Getenv("TRAFFIC_MIGRATIONS_DIR"),
		},
	}

	return config
}

// ValidateConfig ValidateConfig
func ValidateConfig(config Config) {
	if config.RabbitMQ == "" {
		log.Fatalln("Address not provided")
	}

	if config.MonitoringQueue == "" {
		log.Fatalln("MonitoringQueue not provided")
	}

	if config.HotlinkQueue == "" {
		log.Fatalln("HotlinkQueue not provided")
	}
}
