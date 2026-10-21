package config

import (
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		AppPort: "8003", GinMode: "release",
		HTTPReadHeaderTimeout: time.Second, HTTPReadTimeout: 15 * time.Second,
		HTTPWriteTimeout: 15 * time.Second, HTTPIdleTimeout: time.Minute,
		HTTPRequestTimeout: 10 * time.Second,
		DB: DBConfig{
			Driver: "postgres", DSN: "postgres://localhost/test",
			MaxOpenConns: 25, MaxIdleConns: 10,
			ConnMaxLifetime: time.Minute, ConnMaxIdleTime: time.Minute,
		},
		Redis: RedisConfig{
			Addr: "localhost:6379", PoolSize: 10, MinIdleConns: 5,
			MaxRetries: -1, DialTimeout: time.Second,
			ReadTimeout: time.Second, WriteTimeout: time.Second,
		},
		Kafka: KafkaConfig{
			Brokers: []string{"localhost:9092"}, BatchSize: 100,
			BatchTimeout: time.Second, RequiredAcks: "all", Compression: "none",
		},
		OTelServiceName: "test",
	}
}

func TestValidate(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Config)
	}{
		{"invalid port", func(cfg *Config) { cfg.AppPort = "0" }},
		{"invalid log level", func(cfg *Config) { cfg.Log.Level = "fatal" }},
		{"invalid log format", func(cfg *Config) { cfg.Log.Format = "typo" }},
		{"invalid timeout", func(cfg *Config) { cfg.HTTPRequestTimeout = 0 }},
		{"deadline longer than socket timeout", func(cfg *Config) { cfg.HTTPRequestTimeout = time.Minute }},
		{"nested Redis retries", func(cfg *Config) { cfg.Redis.MaxRetries = 3 }},
		{"async publishing", func(cfg *Config) { cfg.Kafka.Async = true }},
		{"no acknowledgement", func(cfg *Config) { cfg.Kafka.RequiredAcks = "none" }},
		{"invalid compression", func(cfg *Config) { cfg.Kafka.Compression = "typo" }},
		{"empty broker", func(cfg *Config) { cfg.Kafka.Brokers = []string{""} }},
		{"invalid pool", func(cfg *Config) { cfg.DB.MaxIdleConns = 100 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfig()
			if err := cfg.Validate(); err != nil {
				t.Fatalf("valid configuration rejected: %v", err)
			}
			test.change(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("expected invalid configuration to be rejected")
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	values := splitCSV("broker1:9092, broker2:9092")
	if len(values) != 2 || values[0] != "broker1:9092" || values[1] != "broker2:9092" {
		t.Fatalf("unexpected brokers: %v", values)
	}
	if values := splitCSV(""); len(values) != 0 {
		t.Fatalf("empty list = %v", values)
	}
}
