package config

import (
	"fmt"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	JWTSecret        string `envconfig:"JWT_SECRET" required:"true"`
	DBLocation       string `envconfig:"DATABASE" required:"true"`
	AdminPassword    string `envconfig:"ADMIN_PASSWORD" required:"true"`
	EditorPassword   string `envconfig:"EDITOR_PASSWORD" required:"true"`
	OperatorPassword string `envconfig:"OPERATOR_PASSWORD" required:"true"`

	ListenHost string `envconfig:"HOST" default:"0.0.0.0"`
	ListenPort string `envconfig:"PORT" default:"8080"`
}

func (c *Config) ListenAddr() string {
	return c.ListenHost + ":" + c.ListenPort
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
