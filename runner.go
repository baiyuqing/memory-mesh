package main

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type txBeginner interface {
	BeginTx(context.Context, *sql.TxOptions) (*sql.Tx, error)
}

func runQuery(ctx context.Context, beginner txBeginner, query string) error {
	tx, err := beginner.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not return rows") {
			_, execErr := tx.ExecContext(ctx, query)
			if execErr != nil {
				_ = tx.Rollback()
				return execErr
			}
			return tx.Commit()
		}
		_ = tx.Rollback()
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	vals := make([]any, len(cols))
	scanArgs := make([]any, len(cols))
	for i := range vals {
		scanArgs[i] = &vals[i]
	}
	for rows.Next() {
		if err := rows.Scan(scanArgs...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

type queryRunner func(context.Context) error

type queryRunnerFactory func(context.Context) (queryRunner, func(), error)

func makeQueryRunnerFactory(cfg config) (queryRunnerFactory, func(), error) {
	if cfg.connectionMode == connectionModePerTxn {
		factory := func(context.Context) (queryRunner, func(), error) {
			runner := func(ctx context.Context) error {
				db, err := sql.Open("mysql", cfg.dsn)
				if err != nil {
					return err
				}
				db.SetMaxOpenConns(1)
				db.SetMaxIdleConns(0)
				defer db.Close()
				return runQuery(ctx, db, cfg.query)
			}
			return runner, func() {}, nil
		}
		return factory, func() {}, nil
	}

	db, err := sql.Open("mysql", cfg.dsn)
	if err != nil {
		return nil, nil, err
	}
	db.SetMaxOpenConns(cfg.concurrency)
	db.SetMaxIdleConns(cfg.concurrency)

	factory := func(ctx context.Context) (queryRunner, func(), error) {
		conn, err := db.Conn(ctx)
		if err != nil {
			return nil, nil, err
		}
		runner := func(ctx context.Context) error {
			return runQuery(ctx, conn, cfg.query)
		}
		return runner, func() { _ = conn.Close() }, nil
	}
	return factory, func() { _ = db.Close() }, nil
}

func worker(ctx context.Context, connectionID int, run queryRunner, m *metrics) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		start := time.Now()
		err := run(ctx)
		if shouldIgnoreQueryResult(ctx, err) {
			return
		}
		m.record(connectionID, time.Since(start), err)
	}
}

func shouldIgnoreQueryResult(ctx context.Context, err error) bool {
	if err == nil || ctx.Err() == nil {
		return false
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
