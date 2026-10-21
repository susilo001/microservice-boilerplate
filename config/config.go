package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	pkgviper "github.com/boilerplate/microservice-boilerplate/pkg/viper"
)

type Config struct {
	AppPort               string
	GinMode               string
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	HTTPRequestTimeout    time.Duration
	DBAutoMigrate         bool

	DB    DBConfig
	Redis RedisConfig
	Kafka KafkaConfig

	OTelServiceName string
}

type DBConfig struct {
	Driver          string // "postgres" | "mysql" | "sqlite" | "sqlserver"
	DSN             string
	ReplicaDSNs     []string // optional; comma-separated via DB_REPLICA_DSNS
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	MaxRetries   int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type KafkaConfig struct {
	Brokers      []string
	BatchSize    int
	BatchTimeout time.Duration
	RequiredAcks string // "all" | "leader" | "none"
	Async        bool
	Compression  string // "none" | "gzip" | "snappy" | "lz4" | "zstd"
}

func Load() Config {
	v := pkgviper.New(".env")

	// Application
	v.SetDefault("APP_PORT", "8003")
	v.SetDefault("GIN_MODE", "debug")
	v.SetDefault("HTTP_READ_HEADER_TIMEOUT", "5s")
	v.SetDefault("HTTP_READ_TIMEOUT", "15s")
	v.SetDefault("HTTP_WRITE_TIMEOUT", "15s")
	v.SetDefault("HTTP_IDLE_TIMEOUT", "60s")
	v.SetDefault("HTTP_REQUEST_TIMEOUT", "10s")
	v.SetDefault("DB_AUTO_MIGRATE", false)

	// Database
	v.SetDefault("DB_DRIVER", "postgres")
	v.SetDefault("DB_DSN", "postgres://postgres:postgres@localhost:5432/productdb?sslmode=disable")
	v.SetDefault("DB_MAX_OPEN_CONNS", 25)
	v.SetDefault("DB_MAX_IDLE_CONNS", 10)
	v.SetDefault("DB_CONN_MAX_LIFETIME", "5m")
	v.SetDefault("DB_CONN_MAX_IDLE_TIME", "5m")

	// Redis
	v.SetDefault("REDIS_ADDR", "localhost:6379")
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_POOL_SIZE", 10)
	v.SetDefault("REDIS_MIN_IDLE_CONNS", 5)
	v.SetDefault("REDIS_MAX_RETRIES", -1)
	v.SetDefault("REDIS_DIAL_TIMEOUT", "5s")
	v.SetDefault("REDIS_READ_TIMEOUT", "3s")
	v.SetDefault("REDIS_WRITE_TIMEOUT", "3s")

	// Kafka
	v.SetDefault("KAFKA_BROKERS", "localhost:9092")
	v.SetDefault("KAFKA_BATCH_SIZE", 100)
	v.SetDefault("KAFKA_BATCH_TIMEOUT", "1s")
	v.SetDefault("KAFKA_REQUIRED_ACKS", "all")
	v.SetDefault("KAFKA_ASYNC", false)
	v.SetDefault("KAFKA_COMPRESSION", "none")
	// OTel
	v.SetDefault("OTEL_SERVICE_NAME", "product-service")

	return Config{
		AppPort:               v.GetString("APP_PORT"),
		GinMode:               v.GetString("GIN_MODE"),
		HTTPReadHeaderTimeout: v.GetDuration("HTTP_READ_HEADER_TIMEOUT"),
		HTTPReadTimeout:       v.GetDuration("HTTP_READ_TIMEOUT"),
		HTTPWriteTimeout:      v.GetDuration("HTTP_WRITE_TIMEOUT"),
		HTTPIdleTimeout:       v.GetDuration("HTTP_IDLE_TIMEOUT"),
		HTTPRequestTimeout:    v.GetDuration("HTTP_REQUEST_TIMEOUT"),
		DBAutoMigrate:         v.GetBool("DB_AUTO_MIGRATE"),

		DB: DBConfig{
			Driver:          v.GetString("DB_DRIVER"),
			DSN:             v.GetString("DB_DSN"),
			ReplicaDSNs:     splitCSV(v.GetString("DB_REPLICA_DSNS")),
			MaxOpenConns:    v.GetInt("DB_MAX_OPEN_CONNS"),
			MaxIdleConns:    v.GetInt("DB_MAX_IDLE_CONNS"),
			ConnMaxLifetime: v.GetDuration("DB_CONN_MAX_LIFETIME"),
			ConnMaxIdleTime: v.GetDuration("DB_CONN_MAX_IDLE_TIME"),
		},

		Redis: RedisConfig{
			Addr:         v.GetString("REDIS_ADDR"),
			Password:     v.GetString("REDIS_PASSWORD"),
			DB:           v.GetInt("REDIS_DB"),
			PoolSize:     v.GetInt("REDIS_POOL_SIZE"),
			MinIdleConns: v.GetInt("REDIS_MIN_IDLE_CONNS"),
			MaxRetries:   v.GetInt("REDIS_MAX_RETRIES"),
			DialTimeout:  v.GetDuration("REDIS_DIAL_TIMEOUT"),
			ReadTimeout:  v.GetDuration("REDIS_READ_TIMEOUT"),
			WriteTimeout: v.GetDuration("REDIS_WRITE_TIMEOUT"),
		},

		Kafka: KafkaConfig{
			Brokers:      splitCSV(v.GetString("KAFKA_BROKERS")),
			BatchSize:    v.GetInt("KAFKA_BATCH_SIZE"),
			BatchTimeout: v.GetDuration("KAFKA_BATCH_TIMEOUT"),
			RequiredAcks: v.GetString("KAFKA_REQUIRED_ACKS"),
			Async:        v.GetBool("KAFKA_ASYNC"),
			Compression:  v.GetString("KAFKA_COMPRESSION"),
		},

		OTelServiceName: v.GetString("OTEL_SERVICE_NAME"),
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	values := strings.Split(value, ",")
	for index := range values {
		values[index] = strings.TrimSpace(values[index])
	}
	return values
}

func (cfg Config) Validate() error {
	port, err := strconv.Atoi(cfg.AppPort)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("APP_PORT must be between 1 and 65535")
	}
	if cfg.GinMode != "debug" && cfg.GinMode != "release" && cfg.GinMode != "test" {
		return fmt.Errorf("invalid GIN_MODE")
	}
	for name, duration := range map[string]time.Duration{
		"HTTP_READ_HEADER_TIMEOUT": cfg.HTTPReadHeaderTimeout,
		"HTTP_READ_TIMEOUT":        cfg.HTTPReadTimeout, "HTTP_WRITE_TIMEOUT": cfg.HTTPWriteTimeout,
		"HTTP_IDLE_TIMEOUT": cfg.HTTPIdleTimeout, "HTTP_REQUEST_TIMEOUT": cfg.HTTPRequestTimeout,
		"DB_CONN_MAX_LIFETIME": cfg.DB.ConnMaxLifetime, "DB_CONN_MAX_IDLE_TIME": cfg.DB.ConnMaxIdleTime,
		"REDIS_DIAL_TIMEOUT": cfg.Redis.DialTimeout, "REDIS_READ_TIMEOUT": cfg.Redis.ReadTimeout,
		"REDIS_WRITE_TIMEOUT": cfg.Redis.WriteTimeout, "KAFKA_BATCH_TIMEOUT": cfg.Kafka.BatchTimeout,
	} {
		if duration <= 0 {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if cfg.HTTPRequestTimeout >= cfg.HTTPWriteTimeout {
		return fmt.Errorf("HTTP_REQUEST_TIMEOUT must be shorter than HTTP_WRITE_TIMEOUT")
	}
	switch strings.ToLower(cfg.DB.Driver) {
	case "postgres", "postgresql", "mysql", "sqlite", "sqlite3", "sqlserver", "mssql":
	default:
		return fmt.Errorf("unsupported DB_DRIVER")
	}
	if strings.TrimSpace(cfg.DB.DSN) == "" || cfg.DB.MaxOpenConns <= 0 ||
		cfg.DB.MaxIdleConns < 0 || cfg.DB.MaxIdleConns > cfg.DB.MaxOpenConns {
		return fmt.Errorf("invalid database DSN or pool settings")
	}
	if strings.TrimSpace(cfg.Redis.Addr) == "" || cfg.Redis.DB < 0 ||
		cfg.Redis.PoolSize <= 0 || cfg.Redis.MinIdleConns < 0 ||
		cfg.Redis.MinIdleConns > cfg.Redis.PoolSize || cfg.Redis.MaxRetries != -1 {
		return fmt.Errorf("invalid Redis settings; REDIS_MAX_RETRIES must be -1 because the adapter owns retries")
	}
	if len(cfg.Kafka.Brokers) == 0 || cfg.Kafka.BatchSize <= 0 {
		return fmt.Errorf("Kafka requires brokers and a positive batch size")
	}
	for _, broker := range cfg.Kafka.Brokers {
		if strings.TrimSpace(broker) == "" {
			return fmt.Errorf("KAFKA_BROKERS contains an empty broker")
		}
	}
	if cfg.Kafka.Async {
		return fmt.Errorf("KAFKA_ASYNC must be false so the product publisher can observe delivery failures")
	}
	if cfg.Kafka.RequiredAcks != "all" && cfg.Kafka.RequiredAcks != "leader" {
		return fmt.Errorf("KAFKA_REQUIRED_ACKS must be all or leader")
	}
	switch cfg.Kafka.Compression {
	case "none", "gzip", "snappy", "lz4", "zstd":
	default:
		return fmt.Errorf("invalid KAFKA_COMPRESSION")
	}
	if strings.TrimSpace(cfg.OTelServiceName) == "" {
		return fmt.Errorf("OTEL_SERVICE_NAME is required")
	}
	return nil
}
