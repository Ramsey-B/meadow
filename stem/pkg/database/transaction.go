package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Gobusters/ectologger"
	"github.com/jmoiron/sqlx"
)

type TxContextKey string

const txStatusKey = TxContextKey("txStatus")
const txKey = TxContextKey("tx-context-key")

type Tx interface {
	IsOpen() bool
	BindNamed(query string, arg any) (string, []any, error)
	Commit(ctx context.Context) error
	DriverName() string
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Get(dest any, query string, args ...any) error
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	MustExec(query string, args ...any) sql.Result
	MustExecContext(ctx context.Context, query string, args ...any) sql.Result
	NamedExec(query string, arg any) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
	NamedQuery(query string, arg any) (*sqlx.Rows, error)
	NamedStmt(stmt *sqlx.NamedStmt) *sqlx.NamedStmt
	NamedStmtContext(ctx context.Context, stmt *sqlx.NamedStmt) *sqlx.NamedStmt
	Prepare(query string) (*sql.Stmt, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	PrepareNamed(query string) (*sqlx.NamedStmt, error)
	PrepareNamedContext(ctx context.Context, query string) (*sqlx.NamedStmt, error)
	Preparex(query string) (*sqlx.Stmt, error)
	PreparexContext(ctx context.Context, query string) (*sqlx.Stmt, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryRowx(query string, args ...any) *sqlx.Row
	QueryRowxContext(ctx context.Context, query string, args ...any) *sqlx.Row
	Queryx(query string, args ...any) (*sqlx.Rows, error)
	QueryxContext(ctx context.Context, query string, args ...any) (*sqlx.Rows, error)
	Rebind(query string) string
	Rollback(ctx context.Context) error
	Select(dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	Stmt(stmt *sql.Stmt) *sql.Stmt
	StmtContext(ctx context.Context, stmt *sql.Stmt) *sql.Stmt
	Stmtx(stmt any) *sqlx.Stmt
	StmtxContext(ctx context.Context, stmt any) *sqlx.Stmt
	Unsafe() *sqlx.Tx
}

// Transaction is a struct that wraps the sqlx.Tx struct and provides additional functionality
type Transaction struct {
	*sqlx.Tx
	logger   ectologger.Logger
	isClosed bool
}

func NewTx(tx *sqlx.Tx, logger ectologger.Logger) Tx {
	return &Transaction{
		Tx:       tx,
		logger:   logger,
		isClosed: false,
	}
}

func GetTx(ctx context.Context, logger ectologger.Logger, db DB, opts *sql.TxOptions) (context.Context, Tx, error) {
	ctxTx, ok := ctx.Value(txKey).(Tx)
	if ok && ctxTx != nil && ctxTx.IsOpen() {
		status, ok := ctx.Value(txStatusKey).(string)
		if ok && status == "open" {
			return ctx, ctxTx, nil
		}
	}

	tx, err := db.BeginTxx(ctx, opts)
	if err != nil {
		logger.WithContext(ctx).WithError(err).Errorf("error while beginning transaction")
		return ctx, nil, fmt.Errorf("error while beginning transaction")
	}

	newTx := NewTx(tx, logger)

	ctx = context.WithValue(ctx, txStatusKey, "open")
	ctx = context.WithValue(ctx, txKey, newTx)
	return ctx, newTx, nil
}

func (t *Transaction) IsOpen() bool {
	return !t.isClosed
}

func (t *Transaction) Rollback(ctx context.Context) error {
	if t.isClosed {
		return nil // do nothing if already committed
	}

	status, ok := ctx.Value(txStatusKey).(string)
	if ok && status == "open" {
		return nil // do nothing. Ctx tx is open and must be closed by the caller
	}

	// Rollback the transaction
	err := t.Tx.Rollback()
	if err != nil {
		t.logger.WithContext(ctx).WithError(err).Errorf("error while rolling back transaction")
		return fmt.Errorf("error while rolling back transaction")
	}

	t.isClosed = true // mark as committed
	return nil
}

func (t *Transaction) Commit(ctx context.Context) error {
	if t.isClosed {
		return nil // do nothing if already committed
	}

	// Commit the transaction
	err := t.Tx.Commit()
	if err != nil {
		t.logger.WithContext(ctx).WithError(err).Errorf("error while committing transaction")
		return fmt.Errorf("error while committing transaction")
	}

	t.isClosed = true // mark as committed

	return nil
}

