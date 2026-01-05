package mappingdefinition

import (
	"database/sql"
	"time"

	"github.com/Ramsey-B/stem/pkg/database"
	"github.com/Ramsey-B/lotus/pkg/fields"
	"github.com/Ramsey-B/lotus/pkg/links"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/lib/pq"
)

func FromMappingDefinition(definition mapping.MappingDefinition) *MappingDefinitionRow {
	return &MappingDefinitionRow{
		ID:              sql.NullString{String: definition.ID, Valid: definition.ID != ""},
		TenantID:        sql.NullString{String: definition.TenantID, Valid: definition.TenantID != ""},
		UserID:          sql.NullString{String: definition.UserID, Valid: definition.UserID != ""},
		Version:         sql.NullInt64{Int64: int64(definition.Version), Valid: definition.Version != 0},
		Key:             sql.NullString{String: definition.Key, Valid: definition.Key != ""},
		Name:            sql.NullString{String: definition.Name, Valid: definition.Name != ""},
		Description:     sql.NullString{String: definition.Description, Valid: definition.Description != ""},
		Tags:            pq.StringArray(definition.Tags),
		IsActive:        sql.NullBool{Bool: definition.IsActive, Valid: true},
		IsDeleted:       sql.NullBool{Bool: definition.IsDeleted, Valid: true},
		CreatedTS:       sql.NullTime{Time: definition.CreatedTS, Valid: definition.CreatedTS != time.Time{}},
		UpdatedTS:       sql.NullTime{Time: definition.UpdatedTS, Valid: definition.UpdatedTS != time.Time{}},
		SourceFields:    database.JSONB[fields.Fields]{Data: definition.SourceFields},
		TargetFields:    database.JSONB[fields.Fields]{Data: definition.TargetFields},
		StepDefinitions: database.JSONB[map[string]models.StepDefinition]{Data: definition.StepDefinitions},
		Links:           database.JSONB[links.Links]{Data: definition.Links},
	}
}

type MappingDefinitionRow struct {
	ID              sql.NullString                                   `db:"id"`
	TenantID        sql.NullString                                   `db:"tenant_id"`
	UserID          sql.NullString                                   `db:"user_id"`
	Version         sql.NullInt64                                    `db:"version"`
	Key             sql.NullString                                   `db:"key"`
	Name            sql.NullString                                   `db:"name"`
	Description     sql.NullString                                   `db:"description"`
	Tags            pq.StringArray                                   `db:"tags"`
	IsActive        sql.NullBool                                     `db:"is_active"`
	IsDeleted       sql.NullBool                                     `db:"is_deleted"`
	CreatedTS       sql.NullTime                                     `db:"created_at"`
	UpdatedTS       sql.NullTime                                     `db:"updated_at"`
	SourceFields    database.JSONB[fields.Fields]                    `db:"source_fields"`
	TargetFields    database.JSONB[fields.Fields]                    `db:"target_fields"`
	StepDefinitions database.JSONB[map[string]models.StepDefinition] `db:"steps"`
	Links           database.JSONB[links.Links]                      `db:"links"`
}

const (
	mappingDefinitionTable = "mapping_definitions"
)

var mappingDefinitionStruct = database.NewStruct(new(MappingDefinitionRow))

func ToMappingDefinition(row *MappingDefinitionRow) mapping.MappingDefinition {
	return mapping.MappingDefinition{
		MappingDefinitionFields: mapping.MappingDefinitionFields{
			ID:          row.ID.String,
			TenantID:    row.TenantID.String,
			UserID:      row.UserID.String,
			Version:     int(row.Version.Int64),
			Key:         row.Key.String,
			Name:        row.Name.String,
			Description: row.Description.String,
			Tags:        []string(row.Tags),
			IsActive:    row.IsActive.Bool,
			IsDeleted:   row.IsDeleted.Bool,
			CreatedTS:   row.CreatedTS.Time,
			UpdatedTS:   row.UpdatedTS.Time,
		},
		SourceFields:    row.SourceFields.Data,
		TargetFields:    row.TargetFields.Data,
		StepDefinitions: row.StepDefinitions.Data,
		Links:           row.Links.Data,
	}
}
