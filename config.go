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
	RabbitMQ        string             `yaml:"rabbitmq"`
	MonitoringQueue string             `yaml:"monitoring_queue"`
	HotlinkQueue    string             `yaml:"hotlink_queue"`
	Rollbar         util.RollbarConfig `yaml:"rollbar"`
	DSN             string             `yaml:"dsn"`
	Migrations      MigrationsConfig   `yaml:"migrations"`
	HTTP            HTTPConfig         `yaml:"http"`
}

// LoadConfig LoadConfig
func LoadConfig() Config {

	config := Config{
		RabbitMQ:        "amqp://guest:guest@" + os.Getenv("TRAFFIC_RABBITMQ_HOST") + ":" + os.Getenv("TRAFFIC_RABBITMQ_PORT") + "/",
		MonitoringQueue: os.Getenv("TRAFFIC_MONITORING_QUEUE"),
		HotlinkQueue:    os.Getenv("TRAFFIC_HOTLINK_QUEUE"),
		Rollbar: util.RollbarConfig{
			Token:       os.Getenv("TRAFFIC_ROLLBAR_TOKEN"),
			Environment: os.Getenv("TRAFFIC_ROLLBAR_ENVIRONMENT"),
			Period:      os.Getenv("TRAFFIC_ROLLBAR_PERIOD"),
		},
		DSN: os.Getenv("TRAFFIC_MYSQL_USERNAME") + ":" + os.Getenv("TRAFFIC_MYSQL_PASSWORD") +
			"@tcp(" + os.Getenv("TRAFFIC_MYSQL_HOST") + ":" + os.Getenv("TRAFFIC_MYSQL_PORT") + ")/" +
			os.Getenv("TRAFFIC_MYSQL_DBNAME") + "?charset=utf8mb4&parseTime=true&loc=UTC",
		HTTP: HTTPConfig{
			Listen: os.Getenv("TRAFFIC_HTTP_LISTEN"),
		},
		Migrations: MigrationsConfig{
			DSN: "mysql://" + os.Getenv("TRAFFIC_MYSQL_USERNAME") + ":" + os.Getenv("TRAFFIC_MYSQL_PASSWORD") +
				"@tcp(" + os.Getenv("TRAFFIC_MYSQL_HOST") + ")/" +
				os.Getenv("TRAFFIC_MYSQL_DBNAME") + "?charset=utf8mb4&parseTime=true&loc=UTC",
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
