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

	// Інтерфейс і порт, які прослуховує сервер. За замовчуванням 0.0.0.0:8080
	// (доступ із усієї локальної мережі).
	ListenHost string `envconfig:"HOST" default:"0.0.0.0"`
	ListenPort string `envconfig:"PORT" default:"8080"`
}

// ListenAddr повертає адресу для http-сервера у форматі host:port.
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
