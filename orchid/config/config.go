package config

import "time"

type Config struct {
	AppName                       string   `env:"APP_NAME" env-default:"orchid-api"`
	Port                          int      `env:"PORT" env-default:"3000"`
	LogLevel                      string   `env:"LOG_LEVEL" env-default:"info"`
	PrettyLogs                    bool     `env:"PRETTY_LOGS" env-default:"false"`
	HttpServerWriteTimeoutSeconds int      `env:"HTTP_SERVER_WRITE_TIMEOUT_SECONDS" env-default:"10"`
	HttpServerReadTimeoutSeconds  int      `env:"HTTP_SERVER_READ_TIMEOUT_SECONDS" env-default:"10"`
	HttpServerIdleTimeoutSeconds  int      `env:"HTTP_SERVER_IDLE_TIMEOUT_SECONDS" env-default:"10"`
	MaxHeaderBytes                int      `env:"HTTP_SERVER_MAX_HEADER_BYTES" env-default:"64000"` // 64KB
	ReadHeaderTimeoutSeconds      int      `env:"HTTP_SERVER_READ_HEADER_TIMEOUT_SECONDS" env-default:"10"`
	TLSMinVersion                 string   `env:"HTTP_SERVER_TLS_MIN_VERSION" env-default:"TLS_1_2"`
	TLSMaxVersion                 string   `env:"HTTP_SERVER_TLS_MAX_VERSION" env-default:"TLS_1_2"`
	AllowOrigins                  []string `env:"HTTP_SERVER_ALLOW_ORIGINS" env-default:"*"`
	AllowMethods                  []string `env:"HTTP_SERVER_ALLOW_METHODS" env-default:"GET,POST,PUT,DELETE"`
	StartupMaxAttempts            int      `env:"STARTUP_MAX_ATTEMPTS" env-default:"5"`

	// Database driver
	DatabaseDriver string `env:"DB_DRIVER" env-default:"postgres"`
	// Database host
	DatabaseHost string `env:"DB_HOST" env-default:""`
	// Database port
	DatabasePort string `env:"DB_PORT" env-default:"5432"`
	// Database user
	DatabaseUserName string `env:"DB_USER_NAME" env-default:""`
	// Database user password
	DatabasePassword string `env:"DB_PASSWORD" env-default:""`
	// Database name
	DatabaseName string `env:"DB_NAME" env-default:"orchid"`
	// Database SQQL Mode
	DatabaseSSLMode string `env:"DB_SQL_MODE" env-default:"disable"`
	// Reconnect Retry Count
	DatabaseReconnectRetryCount int `env:"DB_RECONNECT_RETRY_COUNT" env-default:"3"`
	// Max Open Conns
	DatabaseMaxOpenConns int `env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	// Max Idle Conns
	DatabaseMaxIdleConns int `env:"DB_MAX_IDLE_CONNS" env-default:"10"`
	// Conn Max Lifetime
	DatabaseConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" env-default:"10s"`
	// Migration Folder Path
	DatabaseMigrationFolderPath string `env:"DB_MIGRATION_FOLDER_PATH" env-default:"db/pg"`
	// Database Migration Version
	DatabaseMigrationVersion int `env:"DB_MIGRATION_VERSION" env-default:"0"`
	// Database Migration Force
	DatabaseMigrationForce int `env:"DB_MIGRATION_FORCE" env-default:"0"`
	// Database Migration Auto Rollback
	DatabaseMigrationAutoRollback bool `env:"DB_MIGRATION_AUTO_ROLLBACK" env-default:"true"`

	// Auth Issuer URL
	AuthIssuerURL string `env:"AUTH_ISSUER_URL" env-default:""`
	// Auth Client ID
	AuthClientID string `env:"AUTH_CLIENT_ID" env-default:""`

	// Redis host
	RedisHost string `env:"REDIS_HOST" env-default:"localhost"`
	// Redis port
	RedisPort int `env:"REDIS_PORT" env-default:"6379"`
	// Redis password
	RedisPassword string `env:"REDIS_PASSWORD" env-default:""`
	// Redis database number
	RedisDB int `env:"REDIS_DB" env-default:"0"`

	// Kafka brokers (comma-separated)
	KafkaBrokers string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	// Kafka topic for API responses
	KafkaResponseTopic string `env:"KAFKA_RESPONSE_TOPIC" env-default:"api-responses"`
	// Kafka topic for API errors (responses that are not accepted per step policy)
	KafkaErrorTopic string `env:"KAFKA_ERROR_TOPIC" env-default:"api-errors"`

	// Execution settings
	// Maximum execution time for a plan
	MaxExecutionTime time.Duration `env:"MAX_EXECUTION_TIME" env-default:"5m"`
	// Maximum number of while loop iterations
	MaxLoops int `env:"MAX_LOOPS" env-default:"1000"`
	// Maximum nesting depth for sub-steps
	MaxNestingDepth int `env:"MAX_NESTING_DEPTH" env-default:"5"`

	// Scheduler settings
	// Scheduler poll interval
	SchedulerPollInterval time.Duration `env:"SCHEDULER_POLL_INTERVAL" env-default:"30s"`
	// Enable/disable the scheduler
	SchedulerEnabled bool `env:"SCHEDULER_ENABLED" env-default:"true"`

	// Redis Streams settings
	// Job queue stream name
	RedisStreamsJobQueue string `env:"REDIS_STREAMS_JOB_QUEUE" env-default:"orchid:jobs"`
	// Consumer group name
	RedisStreamsConsumerGroup string `env:"REDIS_STREAMS_CONSUMER_GROUP" env-default:"orchid-workers"`
	// Consumer name (defaults to hostname if empty)
	RedisStreamsConsumerName string `env:"REDIS_STREAMS_CONSUMER_NAME" env-default:""`

	// Tracing settings
	// Enable OTLP tracing export (set to true to send traces to collector)
	OTLPEnabled bool `env:"OTLP_ENABLED" env-default:"false"`
	// OTLP collector endpoint
	OTLPEndpoint string `env:"OTLP_ENDPOINT" env-default:"localhost:4317"`
	// OTLP protocol (grpc or http)
	OTLPProtocol string `env:"OTLP_PROTOCOL" env-default:"grpc"`
	// Disable TLS for OTLP (for local development)
	OTLPInsecure bool `env:"OTLP_INSECURE" env-default:"true"`

	// Auth Enabled - when false, allows X-Tenant-ID and X-User-ID headers for testing
	AuthEnabled bool `env:"AUTH_ENABLED" env-default:"false"`
}
