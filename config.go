package traffic

import (
	"fmt"
	"github.com/autowp/traffic/util"
	"github.com/spf13/viper"
	"log"
)

// HTTPConfig HTTPConfig
type HTTPConfig struct {
	Listen string `yaml:"listen" mapstructure:"listen"`
}

// MigrationsConfig MigrationsConfig
type MigrationsConfig struct {
	DSN string `yaml:"dsn" mapstructure:"dsn"`
	Dir string `yaml:"dir" mapstructure:"dir"`
}

// Config Application config definition
type Config struct {
	RabbitMQ        string            `yaml:"rabbitmq"         mapstructure:"rabbitmq"`
	MonitoringQueue string            `yaml:"monitoring_queue" mapstructure:"monitoring_queue"`
	Sentry          util.SentryConfig `yaml:"sentry"           mapstructure:"sentry"`
	DSN             string            `yaml:"dsn"              mapstructure:"dsn"`
	Migrations      MigrationsConfig  `yaml:"migrations"       mapstructure:"migrations"`
	HTTP            HTTPConfig        `yaml:"http"             mapstructure:"http"`
}

// LoadConfig LoadConfig
func LoadConfig() Config {

	config := Config{}

	viper.SetConfigName("defaults")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	viper.SetConfigName("config")
	err = viper.MergeInConfig()
	if err != nil {
		panic(err)
	}

	err = viper.Unmarshal(&config)
	if err != nil {
		panic(fmt.Errorf("fatal error unmarshal config: %s", err))
	}

	/*config := Config{
		RabbitMQ:        "amqp://guest:guest@" + os.Getenv("TRAFFIC_RABBITMQ_HOST") + ":" + os.Getenv("TRAFFIC_RABBITMQ_PORT") + "/",
		MonitoringQueue: os.Getenv("TRAFFIC_MONITORING_QUEUE"),
		Sentry: util.SentryConfig{
			DSN:         os.Getenv("TRAFFIC_SENTRY_DSN"),
			Environment: os.Getenv("TRAFFIC_SENTRY_ENVIRONMENT"),
		},
		DSN: os.Getenv("TRAFFIC_POSTGRES_DSN"),
		HTTP: HTTPConfig{
			Listen: os.Getenv("TRAFFIC_HTTP_LISTEN"),
		},
		Migrations: MigrationsConfig{
			DSN: os.Getenv("TRAFFIC_MIGRATIONS_DSN"),
			Dir: os.Getenv("TRAFFIC_MIGRATIONS_DIR"),
		},
	}*/

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

}
