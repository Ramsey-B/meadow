package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

type JSONB[T any] struct {
	Data T
}

func (p *JSONB[T]) Scan(src any) error {
	// src is a []byte from pq
	b, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("JSONB.Scan: expected []byte, got %T", src)
	}
	return json.Unmarshal(b, &p.Data)
}

// (Optional) Implement driver.Valuer if you ever want to write back
func (p JSONB[T]) Value() (driver.Value, error) {
	return json.Marshal(p.Data)
}

func (p *JSONB[T]) GetValue() T {
	return p.Data
}

