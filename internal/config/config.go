package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	LogLevel        string
	ShutdownTimeout time.Duration
	DB              Database
}

type Database struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	ConnectTimeout  time.Duration
	QueryTimeout    time.Duration
}

func Load() (*Config, error) {
	var err error
	cfg := &Config{}

	if cfg.HTTPAddr, err = getStrOrErr("HTTP_ADDR"); err != nil {
		return nil, err
	}
	if cfg.LogLevel, err = getStrOrErr("LOG_LEVEL"); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = getDurOrErr("SHUTDOWN_TIMEOUT"); err != nil {
		return nil, err
	}

	if cfg.DB.URL, err = getStrOrErr("DATABASE_URL"); err != nil {
		return nil, err
	}
	if cfg.DB.MaxConns, err = getInt32OrErr("DATABASE_MAX_CONNS"); err != nil {
		return nil, err
	}
	if cfg.DB.MinConns, err = getInt32OrErr("DATABASE_MIN_CONNS"); err != nil {
		return nil, err
	}
	if cfg.DB.MaxConnLifetime, err = getDurOrErr("DATABASE_MAX_CONNECT_LIFETIME"); err != nil {
		return nil, err
	}
	if cfg.DB.ConnectTimeout, err = getDurOrErr("DATABASE_CONNECT_TIMEOUT"); err != nil {
		return nil, err
	}
	if cfg.DB.QueryTimeout, err = getDurOrErr("DATABASE_QUERY_TIMEOUT"); err != nil {
		return nil, err
	}

	return cfg, nil
}

func getStrOrErr(varEnv string) (string, error) {
	if val := os.Getenv(varEnv); val != "" {
		return val, nil
	}
	return "", fmt.Errorf("environment variable %s is required", varEnv)
}

func getDurOrErr(varEnv string) (time.Duration, error) {
	val, err := getStrOrErr(varEnv)
	if err != nil {
		return 0, err
	}

	return time.ParseDuration(val)
}

func getInt32OrErr(varEnv string) (int32, error) {
	val, err := getStrOrErr(varEnv)
	if err != nil {
		return 0, err
	}

	num, err := strconv.ParseInt(val, 10, 32)
	return int32(num), err
}
