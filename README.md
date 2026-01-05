# Shared Infrastructure

Local development infrastructure for Orchid and Lotus.

## Quick Start

```bash
make up
```

## Services

| Service    | Port | Description                |
| ---------- | ---- | -------------------------- |
| PostgreSQL | 5432 | Citus-enabled database     |
| Redis      | 6379 | Cache and message queues   |
| Kafka      | 9092 | Message broker             |
| Kafka UI   | 8080 | Web UI for Kafka debugging |

## Databases

- `orchid` - Orchid API polling service
- `lotus` - Lotus data mapping service

## Commands

```bash
make up              # Start all services
make down            # Stop all services
make clean           # Stop and remove all data
make reset           # Clean and restart
make logs            # Tail all logs
make ps              # Show running services

make psql-orchid     # Connect to orchid database
make psql-lotus      # Connect to lotus database
make redis-cli       # Connect to Redis
```

## Connection Strings

### Orchid

```
DB_HOST=localhost
DB_PORT=5432
DB_NAME=orchid
DB_USER=user
DB_PASSWORD=password
REDIS_HOST=localhost
REDIS_PORT=6379
KAFKA_BROKERS=localhost:9092
```

### Lotus

```
DB_HOST=localhost
DB_PORT=5432
DB_NAME=lotus
DB_USER=user
DB_PASSWORD=password
KAFKA_BROKERS=localhost:9092
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                  meadow                       │
├─────────────────────────────────────────────────────┤
│                                                     │
│  ┌──────────────┐                                   │
│  │   Postgres   │  (Citus single-node mode)         │
│  │   :5432      │                                   │
│  │              │                                   │
│  │  ├─ orchid   │                                   │
│  │  └─ lotus    │                                   │
│  └──────────────┘                                   │
│                                                     │
│  ┌──────────────┐    ┌──────────────────────────┐  │
│  │    Redis     │    │         Kafka            │  │
│  │    :6379     │    │         :9092            │  │
│  └──────────────┘    └──────────────────────────┘  │
│                              │                      │
│                      ┌───────┴───────┐              │
│                      │   Kafka UI    │              │
│                      │   :8080       │              │
│                      └───────────────┘              │
└─────────────────────────────────────────────────────┘
           │                    │
           ▼                    ▼
      ┌─────────┐          ┌─────────┐
      │ Orchid  │ ───────► │  Lotus  │
      │ :3001   │  Kafka   │  :3000  │
      └─────────┘          └─────────┘
```
