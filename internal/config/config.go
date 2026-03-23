package config

import (
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Log      LogConfig
	Transfer TransferConfig
}

type ServerConfig struct {
	Port int
}

type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type LogConfig struct {
	Level string
}

type TransferConfig struct {
	MaxRetries int
}

func Load() (*Config, error) {
	viper.SetDefault("SERVER_PORT", 8080)

	viper.SetDefault("DB_HOST", "localhost")
	viper.SetDefault("DB_PORT", 5432)
	viper.SetDefault("DB_USER", "postgres")
	viper.SetDefault("DB_PASSWORD", "postgres")
	viper.SetDefault("DB_NAME", "transfers")
	viper.SetDefault("DB_SSLMODE", "disable")
	viper.SetDefault("DB_MAX_OPEN", 100)
	viper.SetDefault("DB_MAX_IDLE", 50)
	viper.SetDefault("DB_MAX_LIFETIME", "5m")
	viper.SetDefault("DB_MAX_IDLE_TIME", "1m")

	viper.SetDefault("LOG_LEVEL", "info")

	viper.SetDefault("TRANSFER_MAX_RETRIES", 3)

	viper.AutomaticEnv()

	maxLifetime, err := time.ParseDuration(viper.GetString("DB_MAX_LIFETIME"))
	if err != nil {
		maxLifetime = 5 * time.Minute
	}

	maxIdleTime, err := time.ParseDuration(viper.GetString("DB_MAX_IDLE_TIME"))
	if err != nil {
		maxIdleTime = 1 * time.Minute
	}

	cfg := &Config{
		Server: ServerConfig{
			Port: viper.GetInt("SERVER_PORT"),
		},
		Database: DatabaseConfig{
			Host:            viper.GetString("DB_HOST"),
			Port:            viper.GetInt("DB_PORT"),
			User:            viper.GetString("DB_USER"),
			Password:        viper.GetString("DB_PASSWORD"),
			Name:            viper.GetString("DB_NAME"),
			SSLMode:         viper.GetString("DB_SSLMODE"),
			MaxOpenConns:    viper.GetInt("DB_MAX_OPEN"),
			MaxIdleConns:    viper.GetInt("DB_MAX_IDLE"),
			ConnMaxLifetime: maxLifetime,
			ConnMaxIdleTime: maxIdleTime,
		},
		Log: LogConfig{
			Level: viper.GetString("LOG_LEVEL"),
		},
		Transfer: TransferConfig{
			MaxRetries: viper.GetInt("TRANSFER_MAX_RETRIES"),
		},
	}

	return cfg, nil
}
