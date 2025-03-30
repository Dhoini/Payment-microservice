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
