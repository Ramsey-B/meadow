package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/lotus/pkg/binding"
	"github.com/Ramsey-B/lotus/pkg/kafka"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/stretchr/testify/assert"
)

type noopMappingLoader struct{}

func (n *noopMappingLoader) GetCompiledMapping(ctx context.Context, tenantID, mappingID string) (*mapping.MappingDefinition, error) {
	return nil, nil
}

func TestProcessMessage_EmptyResponseBodyBatchIsNoop(t *testing.T) {
	logger := ectologger.NewEctoLogger(func(_ ectologger.EctoLogMessage) {})
	p := NewProcessor(DefaultProcessorConfig(), binding.NewMatcher(), &noopMappingLoader{}, nil, logger)

	orchidMsg := &kafka.OrchidMessage{
		TenantID:     "t1",
		PlanKey:      "p1",
		ExecutionID:  "e1",
		StepPath:     "root",
		StatusCode:   200,
		ResponseBody: json.RawMessage(`[]`),
	}

	msg := &kafka.ReceivedMessage{
		Headers: kafka.MessageHeaders{TenantID: "t1"},
		Data: map[string]any{
			"response_body": []any{},
		},
		OrchidMessage: orchidMsg,
	}

	results, err := p.ProcessMessage(context.Background(), msg)
	assert.NoError(t, err)
	assert.Len(t, results, 0)
}
