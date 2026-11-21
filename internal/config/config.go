// Package config загружает и валидирует конфигурацию приложения из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const minJWTSecretLen = 32

// Config — конфигурация приложения.
type Config struct {
	DBURL     string
	JWTSecret string
	Port      int
}

// Addr возвращает адрес для http.Server.
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// Load читает конфиг из env и валидирует его.
// Возвращает все найденные ошибки сразу, чтобы не чинить их по одной.
func Load() (Config, error) {
	return load(os.Getenv)
}

func load(getenv func(string) string) (Config, error) {
	var (
		cfg  Config
		errs []error
	)

	cfg.DBURL = strings.TrimSpace(getenv("DB_URL"))
	if cfg.DBURL == "" {
		errs = append(errs, errors.New("DB_URL is required"))
	}

	cfg.JWTSecret = getenv("JWT_SECRET")
	switch {
	case cfg.JWTSecret == "":
		errs = append(errs, errors.New("JWT_SECRET is required"))
	case len(cfg.JWTSecret) < minJWTSecretLen:
		errs = append(errs, fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen))
	}

	portStr := strings.TrimSpace(getenv("PORT"))
	if portStr == "" {
		errs = append(errs, errors.New("PORT is required"))
	} else {
		port, err := strconv.Atoi(portStr)
		if err != nil || port < 1 || port > 65535 {
			errs = append(errs, fmt.Errorf("PORT must be an integer in 1..65535, got %q", portStr))
		}
		cfg.Port = port
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return cfg, nil
}
