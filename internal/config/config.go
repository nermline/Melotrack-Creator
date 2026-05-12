package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	JWTSecret  string `envconfig:"JWT_SECRET" required:"true"`
	DBLocation string `envconfig:"database" required:"true"`
}

func LoadConfig() (*Config, error) {
	cfg := &Config{}

	_ = godotenv.Load()

	err := envconfig.Process("", cfg)
	if err != nil {
		return nil, fmt.Errorf("LoadConfig(): Failed to process .env: %v", err)
	}

	return cfg, nil
}
