package env

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/joho/godotenv"
)

var (
	once       sync.Once
	loadDotenv = godotenv.Load
)

func init() {
	initialize()
}

func initialize() {
	once.Do(func() {
		appEnv := os.Getenv("APP_ENV")
		if appEnv == "" {
			appEnv = "local"
		}
		// silent OK if file doesn't exist — prod envs set vars directly
		_ = loadDotenv(".env." + appEnv)
	})
}

// AppEnv returns the current environment name: "local" | "dev" | "staging" | "prod".
// Defaults to "local" with a warning when APP_ENV is not set.
// Panics on unknown values.
func AppEnv() string {
	v := os.Getenv("APP_ENV")
	if v == "" {
		fmt.Fprintln(os.Stderr, "WARN: APP_ENV not set, defaulting to local")
		return "local"
	}
	switch v {
	case "local", "dev", "staging", "prod":
		return v
	default:
		panic(fmt.Sprintf("APP_ENV must be one of [local dev staging prod], got %q", v))
	}
}

// LogLevel returns "debug" for local/dev and "info" for staging/prod.
func LogLevel() string {
	switch AppEnv() {
	case "local", "dev":
		return "debug"
	default:
		return "info"
	}
}

// DBHost returns the database host. Panics if DB_HOST is not set.
func DBHost() string {
	return mustGet("DB_HOST")
}

// DBPort returns the database port as int. Panics if DB_PORT is not set or not a valid integer.
func DBPort() int {
	raw := mustGet("DB_PORT")
	v, err := strconv.Atoi(raw)
	if err != nil {
		panic(fmt.Sprintf("env: DB_PORT must be an integer, got %q", raw))
	}
	return v
}

// DBUser returns the database user. Panics if DB_USER is not set.
func DBUser() string {
	return mustGet("DB_USER")
}

// DBPassword returns the database password. Panics if DB_PASSWORD is not set.
func DBPassword() string {
	return mustGet("DB_PASSWORD")
}

// DBName returns the database name. Panics if DB_NAME is not set.
func DBName() string {
	return mustGet("DB_NAME")
}

// Port returns the HTTP listen address (e.g. ":8080"). Defaults to ":8080" if PORT is not set.
func Port() string {
	if v := os.Getenv("PORT"); v != "" {
		return v
	}
	return ":8080"
}

func mustGet(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("env: required environment variable %q is not set", key))
	}
	return v
}
