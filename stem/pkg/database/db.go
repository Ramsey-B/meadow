package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/jmoiron/sqlx"
)

type DB interface {
	Begin() (*sql.Tx, error)
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	BeginTxx(ctx context.Context, opts *sql.TxOptions) (*sqlx.Tx, error)
	Beginx() (*sqlx.Tx, error)
	BindNamed(query string, arg any) (string, []any, error)
	Close() error
	Conn(ctx context.Context) (*sql.Conn, error)
	Connx(ctx context.Context) (*sqlx.Conn, error)
	Driver() driver.Driver
	DriverName() string
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Get(dest any, query string, args ...any) error
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	MapperFunc(mf func(string) string)
	MustBegin() *sqlx.Tx
	MustBeginTx(ctx context.Context, opts *sql.TxOptions) *sqlx.Tx
	MustExec(query string, args ...any) sql.Result
	MustExecContext(ctx context.Context, query string, args ...any) sql.Result
	NamedExec(query string, arg any) (sql.Result, error)
	NamedExecContext(ctx context.Context, query string, arg any) (sql.Result, error)
	NamedQuery(query string, arg any) (*sqlx.Rows, error)
	NamedQueryContext(ctx context.Context, query string, arg any) (*sqlx.Rows, error)
	Ping() error
	PingContext(ctx context.Context) error
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
	Select(dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	SetConnMaxIdleTime(d time.Duration)
	SetConnMaxLifetime(d time.Duration)
	SetMaxIdleConns(n int)
	SetMaxOpenConns(n int)
	Stats() sql.DBStats
	Unsafe() *sqlx.DB
	GetTx(ctx context.Context, opts *sql.TxOptions) (context.Context, Tx, error)
}

type DatabaseInstance struct {
	*sqlx.DB
	logger ectologger.Logger
}

func NewDatabaseInstance(db *sqlx.DB, logger ectologger.Logger) DB {
	return &DatabaseInstance{
		DB:     db,
		logger: logger,
	}
}

func (db *DatabaseInstance) GetTx(ctx context.Context, opts *sql.TxOptions) (context.Context, Tx, error) {
	return GetTx(ctx, db.logger, db, opts)
}

