//lint:file-ignore U1000 ignore unused code, it's generated
//nolint:structcheck,unused
package db

import (
	"context"
	"errors"
	"hash/crc64"
	"reflect"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

// DB stores db connection
type DB struct {
	*pg.DB

	crcTable *crc64.Table
}

// New is a function that returns DB as wrapper on postgres connection.
func New(db *pg.DB) DB {
	d := DB{DB: db, crcTable: crc64.MakeTable(crc64.ECMA)}
	return d
}

// Version is a function that returns Postgres version.
func (db *DB) Version() (string, error) {
	var v string
	if _, err := db.QueryOne(pg.Scan(&v), "select version()"); err != nil {
		return "", err
	}

	return v, nil
}

// runInTransaction runs chain of functions in transaction until first error
func (db *DB) runInTransaction(ctx context.Context, fns ...func(*pg.Tx) error) error {
	return db.RunInTransaction(ctx, func(tx *pg.Tx) error {
		for _, fn := range fns {
			if err := fn(tx); err != nil {
				return err
			}
		}
		return nil
	})
}

// RunInLock runs chain of functions in transaction with lock until first error
func (db *DB) RunInLock(ctx context.Context, lockName string, fns ...func(*pg.Tx) error) error {
	lock := int64(crc64.Checksum([]byte(lockName), db.crcTable))

	return db.RunInTransaction(ctx, func(tx *pg.Tx) (err error) {
		if _, err = tx.Exec("select pg_advisory_xact_lock(?) -- ?", lock, lockName); err != nil {
			return
		}

		for _, fn := range fns {
			if err = fn(tx); err != nil {
				return
			}
		}

		return
	})
}

// buildQuery applies all functions to orm query.
func buildQuery(ctx context.Context, db orm.DB, model interface{}, search Searcher, filters []Filter, pager Pager, ops ...OpFunc) *orm.Query {
	q := db.ModelContext(ctx, model)
	for _, filter := range filters {
		filter.Apply(q)
	}

	if reflect.ValueOf(search).IsValid() && !reflect.ValueOf(search).IsNil() { // is it good?
		search.Apply(q)
	}

	q = pager.Apply(q)
	applyOps(q, ops...)

	return q
}

// TxManager provides transaction management capabilities for database operations.
// It allows operations to be performed within an active transaction context
// and falls back to the base connection when no transaction is active.
// Example usage:
//
//	func (m *Manager) runInLock(ctx context.Context, lockName string, fn func(m *Manager) error) error {
//		return m.DB().RunInLock(ctx, lockName, func(tx *pg.Tx) error {
//			txManager := NewManager(*m.DB(), m.client, m.Logger)
//			txManager.sr = txManager.sr.WithTransaction(tx)
//			txManager.SetTx(tx)
//
//			return fn(txManager)
//		})
type TxManager struct {
	dbo *DB
	tx  *pg.Tx
}

// ErrNoActiveTransaction indicates that a transaction operation was attempted
// without an active transaction context.
var ErrNoActiveTransaction = errors.New("no active transaction")

// NewTxManager creates a new TxManager instance with the provided database connection.
func NewTxManager(dbo *DB) TxManager {
	return TxManager{dbo: dbo}
}

// DB returns current database connection.
func (m *TxManager) DB() *DB {
	return m.dbo
}

// Tx returns current transaction. Transaction could be nil.
func (m *TxManager) Tx() *pg.Tx {
	return m.tx
}

// RequireTx returns an error if no transaction is currently active.
// Use this method to ensure operations requiring a transaction context.
func (m *TxManager) RequireTx() error {
	if m.tx == nil {
		return ErrNoActiveTransaction
	}
	return nil
}

// SetTx sets the transaction for the TxManager.
// This allows the manager to operate within a transaction context.
func (m *TxManager) SetTx(tx *pg.Tx) {
	m.tx = tx
}

// Conn returns the current database connection.
// If a transaction is active, returns the transaction; otherwise returns the base DB connection.
func (m *TxManager) Conn() orm.DB {
	if m.tx != nil {
		return m.tx
	}
	return m.dbo
}
