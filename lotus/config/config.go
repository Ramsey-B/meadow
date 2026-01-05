package config

import "time"

type Config struct {
	AppName                       string   `env:"APP_NAME" env-default:"lotus-api"`
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
	DatabaseName string `env:"DB_NAME" env-default:"lotus"`
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

	// Auth Enabled - when false, allows X-Tenant-ID and X-User-ID headers for testing
	AuthEnabled bool `env:"AUTH_ENABLED" env-default:"false"`
	// Auth Issuer URL
	AuthIssuerURL string `env:"AUTH_ISSUER_URL" env-default:""`
	// Auth Client ID
	AuthClientID string `env:"AUTH_CLIENT_ID" env-default:""`

	// Kafka Consumer
	KafkaBrokers          []string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	KafkaInputTopic       string   `env:"KAFKA_INPUT_TOPIC" env-default:"api-responses"`
	KafkaConsumerGroup    string   `env:"KAFKA_CONSUMER_GROUP" env-default:"lotus-consumer"`
	KafkaOutputTopic      string   `env:"KAFKA_OUTPUT_TOPIC" env-default:"mapped-data"`
	KafkaErrorTopic       string   `env:"KAFKA_ERROR_TOPIC" env-default:"mapping-errors"`
	KafkaConsumerEnabled  bool     `env:"KAFKA_CONSUMER_ENABLED" env-default:"true"`

	// Kafka Producer
	KafkaBatchSize    int    `env:"KAFKA_BATCH_SIZE" env-default:"100"`
	KafkaBatchTimeout int    `env:"KAFKA_BATCH_TIMEOUT_MS" env-default:"100"`
	KafkaRequiredAcks int    `env:"KAFKA_REQUIRED_ACKS" env-default:"1"`
	KafkaCompression  string `env:"KAFKA_COMPRESSION" env-default:"snappy"`

	// Processor
	ProcessorWorkerCount     int `env:"PROCESSOR_WORKER_COUNT" env-default:"4"`
	ProcessorTimeoutSeconds  int `env:"PROCESSOR_TIMEOUT_SECONDS" env-default:"30"`
	BindingRefreshIntervalMs int `env:"BINDING_REFRESH_INTERVAL_MS" env-default:"60000"`
	MappingCacheMaxSize      int `env:"MAPPING_CACHE_MAX_SIZE" env-default:"1000"`
	MappingCacheTTLSeconds   int `env:"MAPPING_CACHE_TTL_SECONDS" env-default:"300"`
}
