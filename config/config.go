package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config структура конфигурации приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Logging  LoggingConfig
	GRPC     GRPCConfig
	Stripe   StripeConfig
}

// ServerConfig конфигурация HTTP сервера
type ServerConfig struct {
	Port            string
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
}

// DatabaseConfig конфигурация базы данных
type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// LoggingConfig конфигурация логгера
type LoggingConfig struct {
	Level string
}

// GRPCConfig конфигурация gRPC сервера
type GRPCConfig struct {
	Host     string
	Port     string
	UseTLS   bool
	CertFile string
	KeyFile  string
}

// StripeConfig конфигурация Stripe
type StripeConfig struct {
	APIKey             string
	WebhookSecret      string
	IsTest             bool
	EnableWebhooks     bool
	WebhookEndpointURL string
}

// GetDSN возвращает строку подключения к базе данных
func (c *DatabaseConfig) GetDSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode,
	)
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:            getEnv("PORT", "8080"),
			ReadTimeout:     getEnvAsInt("SERVER_READ_TIMEOUT", 15),
			WriteTimeout:    getEnvAsInt("SERVER_WRITE_TIMEOUT", 15),
			ShutdownTimeout: getEnvAsInt("SERVER_SHUTDOWN_TIMEOUT", 30),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			Database: getEnv("DB_NAME", "payment_service"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Logging: LoggingConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		GRPC: GRPCConfig{
			Host:     getEnv("GRPC_HOST", "0.0.0.0"),
			Port:     getEnv("GRPC_PORT", "50051"),
			UseTLS:   getEnvAsBool("GRPC_USE_TLS", false),
			CertFile: getEnv("GRPC_CERT_FILE", ""),
			KeyFile:  getEnv("GRPC_KEY_FILE", ""),
		},
		Stripe: StripeConfig{
			APIKey:             getEnv("STRIPE_API_KEY", ""),
			WebhookSecret:      getEnv("STRIPE_WEBHOOK_SECRET", ""),
			IsTest:             getEnvAsBool("STRIPE_IS_TEST", true),
			EnableWebhooks:     getEnvAsBool("ENABLE_WEBHOOKS", true),
			WebhookEndpointURL: getEnv("STRIPE_WEBHOOK_ENDPOINT_URL", ""),
		},
	}

	return cfg, nil
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// getEnvAsInt получает значение переменной окружения как int или возвращает значение по умолчанию
func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsBool получает значение переменной окружения как bool или возвращает значение по умолчанию
func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}
