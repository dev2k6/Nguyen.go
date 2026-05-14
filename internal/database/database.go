package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type Config struct {
	Driver      string `yaml:"driver"`
	DSN         string `yaml:"dsn"`
	MaxOpenConn int    `yaml:"max_open_conn"`
	MaxIdleConn int    `yaml:"max_idle_conn"`
	MaxLifetime int    `yaml:"max_lifetime"`
}

type DB struct {
	conn   *sql.DB
	config Config
	mu     sync.RWMutex
}

func New(cfg Config) (*DB, error) {
	if cfg.Driver == "" {
		return nil, fmt.Errorf("database: driver is required")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database: dsn is required")
	}

	conn, err := sql.Open(cfg.Driver, cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("database: failed to open connection: %w", err)
	}

	if cfg.MaxOpenConn > 0 {
		conn.SetMaxOpenConns(cfg.MaxOpenConn)
	}
	if cfg.MaxIdleConn > 0 {
		conn.SetMaxIdleConns(cfg.MaxIdleConn)
	}
	if cfg.MaxLifetime > 0 {
		conn.SetConnMaxLifetime(time.Duration(cfg.MaxLifetime) * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.PingContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("database: ping failed: %w", err)
	}

	return &DB{conn: conn, config: cfg}, nil
}

func (db *DB) Conn() *sql.DB {
	return db.conn
}

func (db *DB) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn.QueryContext(ctx, query, args...)
}

func (db *DB) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn.QueryRowContext(ctx, query, args...)
}

func (db *DB) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.conn.ExecContext(ctx, query, args...)
}

func (db *DB) Begin(ctx context.Context) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, nil)
}

func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return db.conn.BeginTx(ctx, opts)
}

func (db *DB) Close() error {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.conn.Close()
}

func (db *DB) Health(ctx context.Context) error {
	return db.conn.PingContext(ctx)
}
