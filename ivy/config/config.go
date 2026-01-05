package config

import "time"

type Config struct {
	AppName                       string   `env:"APP_NAME" env-default:"ivy-api"`
	Port                          int      `env:"PORT" env-default:"3002"`
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

	// PostgreSQL (Staging Database)
	DatabaseDriver              string        `env:"DB_DRIVER" env-default:"postgres"`
	DatabaseHost                string        `env:"DB_HOST" env-default:""`
	DatabasePort                string        `env:"DB_PORT" env-default:"5432"`
	DatabaseUserName            string        `env:"DB_USER_NAME" env-default:""`
	DatabasePassword            string        `env:"DB_PASSWORD" env-default:""`
	DatabaseName                string        `env:"DB_NAME" env-default:"ivy"`
	DatabaseSSLMode             string        `env:"DB_SQL_MODE" env-default:"disable"`
	DatabaseReconnectRetryCount int           `env:"DB_RECONNECT_RETRY_COUNT" env-default:"3"`
	DatabaseMaxOpenConns        int           `env:"DB_MAX_OPEN_CONNS" env-default:"25"`
	DatabaseMaxIdleConns        int           `env:"DB_MAX_IDLE_CONNS" env-default:"10"`
	DatabaseConnMaxLifetime     time.Duration `env:"DB_CONN_MAX_LIFETIME" env-default:"10s"`
	DatabaseMigrationFolderPath string        `env:"DB_MIGRATION_FOLDER_PATH" env-default:"db/pg"`
	DatabaseMigrationVersion    int           `env:"DB_MIGRATION_VERSION" env-default:"0"`
	DatabaseMigrationForce      int           `env:"DB_MIGRATION_FORCE" env-default:"0"`
	DatabaseMigrationAutoRollback bool        `env:"DB_MIGRATION_AUTO_ROLLBACK" env-default:"true"`

	// Graph Database (Memgraph)
	GraphDBHost     string `env:"GRAPH_DB_HOST" env-default:"localhost"`
	GraphDBPort     int    `env:"GRAPH_DB_PORT" env-default:"7687"`
	GraphDBUser     string `env:"GRAPH_DB_USER" env-default:""`
	GraphDBPassword string `env:"GRAPH_DB_PASSWORD" env-default:""`

	// Auth
	AuthEnabled   bool   `env:"AUTH_ENABLED" env-default:"false"`
	AuthIssuerURL string `env:"AUTH_ISSUER_URL" env-default:""`
	AuthClientID  string `env:"AUTH_CLIENT_ID" env-default:""`

	// Kafka Consumer (Lotus output - ingestion)
	KafkaBrokers         []string `env:"KAFKA_BROKERS" env-default:"localhost:9092"`
	KafkaInputTopic      string   `env:"KAFKA_INPUT_TOPIC" env-default:"mapped-data"`
	KafkaConsumerGroup   string   `env:"KAFKA_CONSUMER_GROUP" env-default:"ivy-consumer"`
	KafkaConsumerEnabled bool     `env:"KAFKA_CONSUMER_ENABLED" env-default:"true"`

	// Kafka Internal Topics (Debezium CDC for staged tables)
	KafkaStagedEntitiesTopic      string `env:"KAFKA_STAGED_ENTITIES_TOPIC" env-default:"ivy.public.staged_entities"`
	KafkaStagedRelationshipsTopic string `env:"KAFKA_STAGED_RELATIONSHIPS_TOPIC" env-default:"ivy.public.staged_relationships"`

	// Kafka Consumer Groups for internal processing
	KafkaMergeConsumerGroup        string `env:"KAFKA_MERGE_CONSUMER_GROUP" env-default:"ivy-merge-consumer"`
	KafkaRelationshipConsumerGroup string `env:"KAFKA_RELATIONSHIP_CONSUMER_GROUP" env-default:"ivy-relationship-consumer"`

	// Kafka Producer settings
	KafkaOutputTopic  string `env:"KAFKA_OUTPUT_TOPIC" env-default:"entity-events"`
	KafkaBatchSize    int    `env:"KAFKA_BATCH_SIZE" env-default:"100"`
	KafkaBatchTimeout int    `env:"KAFKA_BATCH_TIMEOUT_MS" env-default:"100"`
	KafkaRequiredAcks int    `env:"KAFKA_REQUIRED_ACKS" env-default:"1"`
	KafkaCompression  string `env:"KAFKA_COMPRESSION" env-default:"snappy"`

	// Processing
	MatchBatchSize     int     `env:"MATCH_BATCH_SIZE" env-default:"100"`
	MergeWorkerCount   int     `env:"MERGE_WORKER_COUNT" env-default:"4"`
	AutoMergeEnabled   bool    `env:"AUTO_MERGE_ENABLED" env-default:"true"`
	AutoMergeThreshold float64 `env:"AUTO_MERGE_THRESHOLD" env-default:"0.95"`
	ReviewQueueEnabled bool    `env:"REVIEW_QUEUE_ENABLED" env-default:"true"`
}

