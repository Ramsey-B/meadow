package database

import (
	"fmt"
	"strings"

	"github.com/huandu/go-sqlbuilder"
)

func Excluded(column string) any {
	return sqlbuilder.Raw(fmt.Sprintf("EXCLUDED.%s", column))
}

type InsertBuilder struct {
	*sqlbuilder.InsertBuilder
}

func NewInsertBuilder() *InsertBuilder {
	return &InsertBuilder{
		sqlbuilder.PostgreSQL.NewInsertBuilder(),
	}
}

func (b *InsertBuilder) OnConflict(columns ...string) *UpdateBuilder {
	ub := NewUpdateBuilder()
	b.SQL(fmt.Sprintf("ON CONFLICT (%s) DO UPDATE %s", strings.Join(columns, ", "), b.Var(ub)))

	return ub
}

func (b *InsertBuilder) OnConflictDoNothing() *InsertBuilder {
	b.SQL("ON CONFLICT DO NOTHING")
	return b
}

func (ib *InsertBuilder) Build() (sql string, args []interface{}) {
	return ib.InsertBuilder.Build()
}
func (ib *InsertBuilder) BuildWithFlavor(flavor sqlbuilder.Flavor, initialArg ...interface{}) (sql string, args []interface{}) {
	return ib.InsertBuilder.BuildWithFlavor(flavor, initialArg...)
}
func (ib *InsertBuilder) Cols(col ...string) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.Cols(col...)}
}
func (ib *InsertBuilder) Flavor() sqlbuilder.Flavor {
	return ib.InsertBuilder.Flavor()
}
func (ib *InsertBuilder) InsertIgnoreInto(table string) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.InsertIgnoreInto(table)}
}
func (ib *InsertBuilder) InsertInto(table string) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.InsertInto(table)}
}
func (ib *InsertBuilder) NumValue() int {
	return ib.InsertBuilder.NumValue()
}
func (ib *InsertBuilder) ReplaceInto(table string) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.ReplaceInto(table)}
}
func (ib *InsertBuilder) Returning(col ...string) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.Returning(col...)}
}
func (ib *InsertBuilder) SetFlavor(flavor sqlbuilder.Flavor) (old sqlbuilder.Flavor) {
	return ib.InsertBuilder.SetFlavor(flavor)
}
func (ib *InsertBuilder) String() string {
	return ib.InsertBuilder.String()
}
func (ib *InsertBuilder) Values(value ...interface{}) *InsertBuilder {
	return &InsertBuilder{ib.InsertBuilder.Values(value...)}
}
func (ib *InsertBuilder) Var(arg interface{}) string {
	return ib.InsertBuilder.Var(arg)
}

type UpdateBuilder struct {
	*sqlbuilder.UpdateBuilder
}

func NewUpdateBuilder() *UpdateBuilder {
	return &UpdateBuilder{sqlbuilder.PostgreSQL.NewUpdateBuilder()}
}

type DeleteBuilder struct {
	*sqlbuilder.DeleteBuilder
}

func NewDeleteBuilder() *DeleteBuilder {
	return &DeleteBuilder{sqlbuilder.PostgreSQL.NewDeleteBuilder()}
}

type SelectBuilder struct {
	*sqlbuilder.SelectBuilder
}

func NewSelectBuilder() *SelectBuilder {
	return &SelectBuilder{sqlbuilder.PostgreSQL.NewSelectBuilder()}
}

type Struct struct {
	*sqlbuilder.Struct
}

func (s *Struct) SelectFrom(table string) *SelectBuilder {
	return &SelectBuilder{s.Struct.SelectFrom(table)}
}

func (s *Struct) InsertInto(table string, v ...any) *InsertBuilder {
	return &InsertBuilder{s.Struct.InsertInto(table, v...)}
}

func (s *Struct) Update(table string, v any) *UpdateBuilder {
	return &UpdateBuilder{s.Struct.Update(table, v)}
}

func (s *Struct) DeleteFrom(table string) *DeleteBuilder {
	return &DeleteBuilder{s.Struct.DeleteFrom(table)}
}

func NewStruct(v any) *Struct {
	builder := sqlbuilder.NewStruct(v).For(sqlbuilder.PostgreSQL)
	return &Struct{builder}
}

