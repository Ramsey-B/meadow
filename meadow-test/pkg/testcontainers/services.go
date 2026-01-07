package testcontainers

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ServiceManager manages Testcontainers for Meadow services
type ServiceManager struct {
	ctx context.Context

	// Infrastructure
	postgres  testcontainers.Container
	kafka     testcontainers.Container
	redis     testcontainers.Container
	memgraph  testcontainers.Container
	zookeeper testcontainers.Container

	// Meadow services
	mocks  testcontainers.Container
	orchid testcontainers.Container
	lotus  testcontainers.Container
	ivy    testcontainers.Container

	// Network
	network testcontainers.Network

	// URLs for services
	PostgresURL  string
	KafkaBrokers []string
	RedisURL     string
	MemgraphURL  string
	MocksURL     string
	OrchidURL    string
	LotusURL     string
	IvyURL       string
}

// NewServiceManager creates a new service manager
func NewServiceManager(ctx context.Context) *ServiceManager {
	return &ServiceManager{
		ctx: ctx,
	}
}

// StartInfrastructure starts the core infrastructure (PostgreSQL, Kafka, Redis, Memgraph)
func (sm *ServiceManager) StartInfrastructure() error {
	// Create network
	network, err := testcontainers.GenericNetwork(sm.ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           "meadow-test-network",
			CheckDuplicate: true,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	sm.network = network

	// Start Zookeeper (required for Kafka)
	if err := sm.startZookeeper(); err != nil {
		return fmt.Errorf("failed to start Zookeeper: %w", err)
	}

	// Start Kafka
	if err := sm.startKafka(); err != nil {
		return fmt.Errorf("failed to start Kafka: %w", err)
	}

	// Start PostgreSQL
	if err := sm.startPostgres(); err != nil {
		return fmt.Errorf("failed to start PostgreSQL: %w", err)
	}

	// Start Redis
	if err := sm.startRedis(); err != nil {
		return fmt.Errorf("failed to start Redis: %w", err)
	}

	// Start Memgraph
	if err := sm.startMemgraph(); err != nil {
		return fmt.Errorf("failed to start Memgraph: %w", err)
	}

	return nil
}

// startZookeeper starts Zookeeper container
func (sm *ServiceManager) startZookeeper() error {
	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-zookeeper:7.5.0",
		ExposedPorts: []string{"2181/tcp"},
		Env: map[string]string{
			"ZOOKEEPER_CLIENT_PORT": "2181",
			"ZOOKEEPER_TICK_TIME":   "2000",
		},
		Networks: []string{"meadow-test-network"},
		WaitingFor: wait.ForLog("binding to port").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.zookeeper = container
	return nil
}

// startKafka starts Kafka container
func (sm *ServiceManager) startKafka() error {
	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:7.5.0",
		ExposedPorts: []string{"9092/tcp", "9093/tcp"},
		Env: map[string]string{
			"KAFKA_BROKER_ID":                        "1",
			"KAFKA_ZOOKEEPER_CONNECT":                "zookeeper:2181",
			"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://localhost:9092,PLAINTEXT_INTERNAL://kafka:9093",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "PLAINTEXT:PLAINTEXT,PLAINTEXT_INTERNAL:PLAINTEXT",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":    "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR": "1",
		},
		Networks: []string{"meadow-test-network"},
		WaitingFor: wait.ForLog("started (kafka.server.KafkaServer)").
			WithStartupTimeout(120 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.kafka = container

	// Get host and port
	host, err := container.Host(sm.ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(sm.ctx, "9092")
	if err != nil {
		return err
	}

	sm.KafkaBrokers = []string{fmt.Sprintf("%s:%s", host, port.Port())}
	return nil
}

// startPostgres starts PostgreSQL container
func (sm *ServiceManager) startPostgres() error {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "user",
			"POSTGRES_PASSWORD": "password",
			"POSTGRES_DB":       "meadow",
		},
		Networks: []string{"meadow-test-network"},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.postgres = container

	// Get connection URL
	host, err := container.Host(sm.ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(sm.ctx, "5432")
	if err != nil {
		return err
	}

	sm.PostgresURL = fmt.Sprintf("postgres://user:password@%s:%s/meadow?sslmode=disable", host, port.Port())
	return nil
}

// startRedis starts Redis container
func (sm *ServiceManager) startRedis() error {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		Networks:     []string{"meadow-test-network"},
		WaitingFor: wait.ForLog("Ready to accept connections").
			WithStartupTimeout(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.redis = container

	host, err := container.Host(sm.ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(sm.ctx, "6379")
	if err != nil {
		return err
	}

	sm.RedisURL = fmt.Sprintf("%s:%s", host, port.Port())
	return nil
}

// startMemgraph starts Memgraph container
func (sm *ServiceManager) startMemgraph() error {
	req := testcontainers.ContainerRequest{
		Image:        "memgraph/memgraph:latest",
		ExposedPorts: []string{"7687/tcp"},
		Env: map[string]string{
			"MEMGRAPH_USER":     "user",
			"MEMGRAPH_PASSWORD": "password",
		},
		Networks: []string{"meadow-test-network"},
		WaitingFor: wait.ForLog("Server is fully armed and operational").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.memgraph = container

	host, err := container.Host(sm.ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(sm.ctx, "7687")
	if err != nil {
		return err
	}

	sm.MemgraphURL = fmt.Sprintf("bolt://%s:%s", host, port.Port())
	return nil
}

// StartMocks starts the mock API service
func (sm *ServiceManager) StartMocks() error {
	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    "../mocks",
			Dockerfile: "Dockerfile",
		},
		ExposedPorts: []string{"9000/tcp"},
		Networks:     []string{"meadow-test-network"},
		WaitingFor: wait.ForHTTP("/health").
			WithPort("9000/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(sm.ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return err
	}

	sm.mocks = container

	host, err := container.Host(sm.ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(sm.ctx, "9000")
	if err != nil {
		return err
	}

	sm.MocksURL = fmt.Sprintf("http://%s:%s", host, port.Port())
	return nil
}

// Cleanup stops and removes all containers
func (sm *ServiceManager) Cleanup() error {
	containers := []testcontainers.Container{
		sm.ivy,
		sm.lotus,
		sm.orchid,
		sm.mocks,
		sm.memgraph,
		sm.redis,
		sm.postgres,
		sm.kafka,
		sm.zookeeper,
	}

	for _, container := range containers {
		if container != nil {
			if err := container.Terminate(sm.ctx); err != nil {
				// Log but don't fail on cleanup errors
				fmt.Printf("Warning: failed to terminate container: %v\n", err)
			}
		}
	}

	if sm.network != nil {
		if err := sm.network.Remove(sm.ctx); err != nil {
			fmt.Printf("Warning: failed to remove network: %v\n", err)
		}
	}

	return nil
}

// IsInfrastructureReady checks if infrastructure is ready
func (sm *ServiceManager) IsInfrastructureReady() bool {
	return sm.postgres != nil && sm.kafka != nil && sm.redis != nil
}
