package config

import (
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
	v.SetDefault("REDIS_MAX_RETRIES", 3)
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

		DB: DBConfig{
			Driver:          v.GetString("DB_DRIVER"),
			DSN:             v.GetString("DB_DSN"),
			ReplicaDSNs:     v.GetStringSlice("DB_REPLICA_DSNS"),
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
			Brokers:      v.GetStringSlice("KAFKA_BROKERS"),
			BatchSize:    v.GetInt("KAFKA_BATCH_SIZE"),
			BatchTimeout: v.GetDuration("KAFKA_BATCH_TIMEOUT"),
			RequiredAcks: v.GetString("KAFKA_REQUIRED_ACKS"),
			Async:        v.GetBool("KAFKA_ASYNC"),
			Compression:  v.GetString("KAFKA_COMPRESSION"),
		},

		OTelServiceName: v.GetString("OTEL_SERVICE_NAME"),
	}
}
