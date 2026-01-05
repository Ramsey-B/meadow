# Kafka Library Recommendation for Orchid

## Recommendation: `github.com/segmentio/kafka-go`

**Best choice for Orchid because:**
- ✅ Pure Go (no cgo dependencies)
- ✅ Simple, idiomatic Go API
- ✅ Excellent for producers (what Orchid needs)
- ✅ Good performance
- ✅ Actively maintained
- ✅ Easy to integrate
- ✅ Good error handling

## Library Comparison

### 1. **kafka-go** (Segment) ⭐ RECOMMENDED
**Package**: `github.com/segmentio/kafka-go`

**Pros:**
- Pure Go implementation (no cgo)
- Simple, clean API
- Excellent for producers
- Good performance
- Well-documented
- Actively maintained
- Easy to use with Go's standard patterns
- Good error handling
- Supports all Kafka features needed

**Cons:**
- Slightly less performant than librdkafka-based solutions
- Less feature-complete than Confluent client (but has everything needed)

**Best for:** Most Go projects, especially when you need a producer

**Example:**
```go
w := kafka.NewWriter(kafka.WriterConfig{
    Brokers: []string{"localhost:9092"},
    Topic:   "orchid-data",
})

err := w.WriteMessages(context.Background(),
    kafka.Message{
        Key:   []byte("key"),
        Value: []byte("value"),
    },
)
```

---

### 2. **franz-go** (Twmb)
**Package**: `github.com/twmb/franz-go`

**Pros:**
- Pure Go implementation
- Very high performance
- Feature-complete
- Modern API
- Good for both producers and consumers
- Actively maintained

**Cons:**
- More complex API than kafka-go
- Steeper learning curve
- Less commonly used (smaller community)

**Best for:** High-performance scenarios, when you need maximum performance

---

### 3. **Confluent Kafka Go**
**Package**: `github.com/confluentinc/confluent-kafka-go`

**Pros:**
- Official Confluent library
- Very high performance (uses librdkafka C library)
- Feature-complete
- Production-tested at scale
- Excellent documentation

**Cons:**
- Requires cgo (C dependencies)
- Requires librdkafka to be installed
- More complex deployment (Docker images need C libraries)
- Can be harder to cross-compile
- More complex API

**Best for:** Maximum performance requirements, when you can handle cgo dependencies

---

### 4. **Sarama**
**Package**: `github.com/IBM/sarama`

**Pros:**
- Pure Go
- Very feature-complete
- Widely used
- Good for consumers

**Cons:**
- Complex API
- More verbose
- Overkill for simple producer use case
- Less maintained recently

**Best for:** Complex consumer scenarios, when you need advanced features

---

### 5. **Goka**
**Package**: `github.com/lovoo/goka`

**Pros:**
- Stream processing framework
- Built on Sarama
- Good for complex stream processing

**Cons:**
- Overkill for simple producer
- More complex
- Framework, not just a client

**Best for:** Stream processing applications, not simple producers

---

## For Orchid's Use Case

**Orchid needs:**
- ✅ Simple producer (emitting raw API responses)
- ✅ Reliable message delivery
- ✅ Good error handling
- ✅ Easy integration
- ✅ No cgo dependencies (simpler deployment)
- ✅ Good performance (but not extreme)

**kafka-go is perfect because:**
1. Simple producer API - exactly what's needed
2. Pure Go - no deployment complications
3. Good enough performance for data emission
4. Easy to integrate with existing codebase
5. Well-maintained and stable

---

## Implementation Example

```go
package kafka

import (
    "context"
    "encoding/json"
    
    "github.com/segmentio/kafka-go"
    "github.com/Gobusters/ectologger"
)

type Producer struct {
    writer *kafka.Writer
    logger ectologger.Logger
}

func NewProducer(brokers []string, topic string, logger ectologger.Logger) *Producer {
    return &Producer{
        writer: &kafka.Writer{
            Addr:     kafka.TCP(brokers...),
            Topic:    topic,
            Balancer: &kafka.LeastBytes{}, // Good default
            Async:    false, // Synchronous for reliability
        },
        logger: logger,
    }
}

func (p *Producer) Emit(ctx context.Context, key string, data []byte) error {
    msg := kafka.Message{
        Key:   []byte(key),
        Value: data,
        Time:  time.Now(),
    }
    
    err := p.writer.WriteMessages(ctx, msg)
    if err != nil {
        p.logger.WithError(err).Error("Failed to emit message to Kafka")
        return err
    }
    
    return nil
}

func (p *Producer) Close() error {
    return p.writer.Close()
}
```

---

## Configuration Options

kafka-go supports all the configuration you'll need:

```go
writer := kafka.NewWriter(kafka.WriterConfig{
    Brokers:  []string{"broker1:9092", "broker2:9092"},
    Topic:    "orchid-data",
    Balancer: &kafka.LeastBytes{}, // or Hash, RoundRobin, etc.
    
    // Reliability
    RequiredAcks: kafka.RequireAll, // Wait for all replicas
    Async:        false,             // Synchronous writes
    
    // Performance
    BatchSize:    100,               // Batch messages
    BatchBytes:   1048576,           // 1MB batches
    BatchTimeout: 10 * time.Millisecond,
    
    // Error handling
    ErrorLogger:  errorLogger,
    Logger:       infoLogger,
    
    // Compression
    CompressionCodec: kafka.Snappy,
    
    // Timeouts
    WriteTimeout: 10 * time.Second,
    ReadTimeout:  10 * time.Second,
})
```

---

## Alternative: If You Need Maximum Performance

If you later find you need maximum performance, consider **franz-go**:

```go
import "github.com/twmb/franz-go/pkg/kgo"

client, err := kgo.NewClient(
    kgo.SeedBrokers("broker1:9092", "broker2:9092"),
    kgo.DefaultProduceTopic("orchid-data"),
    kgo.RequiredAcks(kgo.AllISRAcks()), // Wait for all replicas
)
```

But for most use cases, kafka-go is the better choice due to simplicity.

---

## Recommendation Summary

**Use `github.com/segmentio/kafka-go`** for Orchid because:
1. Simple producer API - perfect for emitting data
2. Pure Go - no deployment headaches
3. Good performance - sufficient for data emission
4. Easy integration - fits well with existing codebase
5. Well-maintained - stable and reliable

**Install:**
```bash
go get github.com/segmentio/kafka-go
```

**Update IMPLEMENTATION_PLAN.md:**
- Change Kafka library recommendation to `github.com/segmentio/kafka-go`
- Update Phase 1.3 with kafka-go specific implementation details

