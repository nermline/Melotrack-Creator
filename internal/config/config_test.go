package config

import (
	"os"
	"testing"
)

var requiredKeys = []string{"JWT_SECRET", "DATABASE", "ADMIN_PASSWORD", "EDITOR_PASSWORD", "OPERATOR_PASSWORD"}
var optionalKeys = []string{"HOST", "PORT"}

func clearEnv() {
	for _, k := range append(append([]string{}, requiredKeys...), optionalKeys...) {
		os.Unsetenv(k)
	}
}

func setRequired() {
	os.Setenv("JWT_SECRET", "secret")
	os.Setenv("DATABASE", "./test.db")
	os.Setenv("ADMIN_PASSWORD", "adminpw")
	os.Setenv("EDITOR_PASSWORD", "editorpw")
	os.Setenv("OPERATOR_PASSWORD", "operpw")
}

func TestLoadConfig_Defaults(t *testing.T) {
	clearEnv()
	defer clearEnv()
	setRequired()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.JWTSecret != "secret" || cfg.DBLocation != "./test.db" {
		t.Errorf("required fields not parsed: %+v", cfg)
	}
	if cfg.ListenHost != "0.0.0.0" || cfg.ListenPort != "8080" {
		t.Errorf("expected default host/port, got %s:%s", cfg.ListenHost, cfg.ListenPort)
	}
	if cfg.ListenAddr() != "0.0.0.0:8080" {
		t.Errorf("ListenAddr() = %q", cfg.ListenAddr())
	}
}

func TestLoadConfig_HostPortOverride(t *testing.T) {
	clearEnv()
	defer clearEnv()
	setRequired()
	os.Setenv("HOST", "127.0.0.1")
	os.Setenv("PORT", "9999")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.ListenAddr() != "127.0.0.1:9999" {
		t.Errorf("ListenAddr() = %q, want 127.0.0.1:9999", cfg.ListenAddr())
	}
}

func TestLoadConfig_MissingRequired(t *testing.T) {
	clearEnv()
	defer clearEnv()

	if _, err := LoadConfig(); err == nil {
		t.Error("expected error when required env vars are missing")
	}
}
