package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"os"
)

// Config представляет структуру конфигурации для приложения.
type Config struct {
	App struct {
		Port string `mapstructure:"port"`
		Env  string `mapstructure:"env"`
	} `mapstructure:"app"`
	Database struct {
		DSN string `mapstructure:"dsn"`
	} `mapstructure:"database"`
	Redis struct {
		Addr     string `mapstructure:"addr"`
		Password string `mapstructure:"password"`
		DB       int    `mapstructure:"db"`
	} `mapstructure:"redis"`
	Kafka struct {
		Brokers []string `mapstructure:"brokers"`
		Topic   string   `mapstructure:"topic"`
		GroupID string   `mapstructure:"groupId"`
	} `mapstructure:"kafka"`
	Stripe struct {
		APIKey        string `mapstructure:"apiKey"`
		WebhookSecret string `mapstructure:"webhookSecret"` // Добавим позже
	} `mapstructure:"stripe"`
	GRPC struct {
		Port string `mapstructure:"port"`
	} `mapstructure:"grpc"`
	Auth struct {
		JWTSecret string `mapstructure:"jwtSecret"`
	} `mapstructure:"auth"`
}

// LoadConfig загружает конфигурацию из файла или переменных окружения.
func LoadConfig(path string) (*Config, error) {
	if os.Getenv("APP_ENV") != "production" {
		err := godotenv.Load(path)
		if err != nil {
			return nil, err
		}
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	viper.AutomaticEnv() // Чтение переменных окружения

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
