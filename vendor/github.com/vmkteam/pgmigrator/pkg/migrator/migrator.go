package migrator

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type Migrator struct {
	db       *pg.DB
	cfg      Config
	rootDir  string // patches
	fileMask *regexp.Regexp
}

func NewMigrator(db *pg.DB, cfg Config, rootDir string) *Migrator {
	m := &Migrator{
		db:       db,
		cfg:      cfg,
		rootDir:  rootDir,
		fileMask: regexp.MustCompile(cfg.FileMask),
	}

	if db != nil {
		m.db = db.WithParam("migrationTable", pg.Ident(cfg.Table))
	}

	return m
}

// writeMigrationToDB inserts log that migration was completed in postgres
func writeMigrationToDB(ctx context.Context, mg Migration, tx *pg.Tx, start time.Time) error {
	finish := time.Now()
	pm := mg.ToDB()
	pm.StartedAt = start
	pm.FinishedAt = &finish

	if _, err := tx.ModelContext(ctx, pm).Insert(); err != nil {
		return fmt.Errorf(`add new migration "%s" failed: %w`, mg.Filename, err)
	}
	return nil
}

// readAllFiles read files from migrator root dir and return its filenames
func (m *Migrator) readAllFiles() ([]string, error) {
	dir, err := os.Open(m.rootDir)
	if err != nil {
		return nil, fmt.Errorf("open dir failed: %w", err)
	}
	defer dir.Close()

	files, err := dir.ReadDir(-1)
	if err != nil {
		return nil, fmt.Errorf("read files failed: %w", err)
	}

	var filenames []string
	for _, f := range files {
		if f.IsDir() || !m.fileMask.MatchString(f.Name()) {
			continue
		} else if strings.HasSuffix(f.Name(), "MANUAL.sql") {
			// skip manual migrations
			continue
		}

		filenames = append(filenames, f.Name())
	}

	sort.Strings(filenames)

	return filenames, nil
}

// removeCompleted remove completed migration filenames from all list
func (m *Migrator) removeCompleted(all, completed []string) (res []string) {
	completedMapping := make(map[string]struct{})
	for _, n := range completed {
		completedMapping[n] = struct{}{}
	}

	for _, f := range all {
		if _, ok := completedMapping[f]; !ok {
			res = append(res, f)
		}
	}

	return
}

// Plan reads filenames from migrator root dir, fetch completed filenames from db, compare its and returns .
func (m *Migrator) Plan(ctx context.Context) ([]string, error) {
	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return nil, err
	}

	// read all files
	filenames, err := m.readAllFiles()
	if err != nil {
		return nil, err
	} else if len(filenames) == 0 {
		return nil, nil
	}

	// fetch completed migrations from db
	var completed []string
	_, err = m.db.QueryContext(ctx, &completed, `select "filename" from ? where "filename" in (?)`, pg.Ident(m.cfg.Table), pg.In(filenames))
	if err != nil {
		return nil, err
	}

	// compare and plan
	return m.removeCompleted(filenames, completed), nil
}

// Run run migrations from files, apply transactional and non transactional
func (m *Migrator) Run(ctx context.Context, filenames []string, chCurrentFile chan string) error {
	defer close(chCurrentFile)

	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return err
	}

	// prepare migrations
	mm, err := m.newMigrations(filenames)
	if err != nil {
		return fmt.Errorf("prepare migrations failed: %w", err)
	}

	// apply migrations
	for _, mg := range mm {
		chCurrentFile <- mg.Filename
		if mg.Transactional {
			err = m.applyMigration(ctx, mg)
		} else {
			err = m.applyNonTransactionalMigration(ctx, mg)
		}

		if err != nil {
			return fmt.Errorf("%s: %w", mg.Filename, err)
		}
	}

	return err
}

// newMigrations create Migrations from filenames
func (m *Migrator) newMigrations(filenames []string) (Migrations, error) {
	var mm Migrations
	for _, filename := range filenames {
		mg, err := NewMigration(m.rootDir, filename)
		if err != nil {
			return nil, fmt.Errorf("%s open failed: %w", mg.Filename, err)
		}

		mm = append(mm, mg)
	}

	return mm, nil
}

func finishTxOnErr(tx *pg.Tx, err error) error {
	var er error
	if err != nil {
		er = tx.Rollback()
	} else {
		er = tx.Commit()
	}

	if er != nil {
		err = fmt.Errorf("failed to finish transaction er=%w: %w", er, err)
	}

	return err
}

// applyMigration apply transactional migration
func (m *Migrator) applyMigration(ctx context.Context, mg Migration) (err error) {
	var tx *pg.Tx
	tx, err = m.db.Begin()
	if err != nil {
		return fmt.Errorf(`begin transaction failed: %w`, err)
	}

	defer func() {
		err = finishTxOnErr(tx, err)
	}()

	if err = m.setStatementTimeout(ctx, tx); err != nil {
		return err
	}

	// run
	start := time.Now()
	if _, err = tx.ExecContext(ctx, string(mg.Data)); err != nil {
		return fmt.Errorf(`apply migration failed: %w`, err)
	}

	return writeMigrationToDB(ctx, mg, tx, start)
}

// setStatementTimeout set statement timeout to transaction connection
func (m *Migrator) setStatementTimeout(ctx context.Context, tx orm.DB) error {
	if m.cfg.StatementTimeout == "" {
		return nil
	}

	if _, err := tx.ExecContext(ctx, `set statement_timeout to ?`, m.cfg.StatementTimeout); err != nil {
		return fmt.Errorf(`set statement timeout failed: %w`, err)
	}

	return nil
}

// applyNonTransactionalMigration apply non-transactional migration
func (m *Migrator) applyNonTransactionalMigration(ctx context.Context, mg Migration) error {
	if err := m.setStatementTimeout(ctx, m.db); err != nil {
		return err
	}
	// insert into pgMigrations
	pm := mg.ToDB()
	pm.StartedAt = time.Now()
	if _, err := m.db.ModelContext(ctx, pm).Insert(); err != nil {
		return fmt.Errorf(`add new migration failed: %w`, err)
	}

	// run
	if _, err := m.db.ExecContext(ctx, string(mg.Data)); err != nil {
		return fmt.Errorf(`apply migration failed: %w`, err)
	}

	// update pgMigrations
	now := time.Now()
	pm.FinishedAt = &now
	if _, err := m.db.ModelContext(ctx, pm).Column("finishedAt").WherePK().Update(); err != nil {
		return fmt.Errorf(`update finishedAt migration failed: %w`, err)
	}

	return nil
}

// DryRun tries to apply migrations. Runs migrations inside single transaction and always rolls back it
// returns err, if apply done with error or if non-transactional migration was found
func (m *Migrator) DryRun(ctx context.Context, filenames []string, chCurrentFile chan string) error {
	defer close(chCurrentFile)

	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return err
	}

	// prepare migrations
	mm, err := m.newMigrations(filenames)
	if err != nil {
		return fmt.Errorf("prepare migrations failed: %w", err)
	} else if t, ok := mm.FirstNonTransactional(); ok {
		// check NONTR
		return fmt.Errorf(`non transactional migration found "%s", run all migrations before it, please`, t.Filename)
	}

	// dryRun migrations
	if err = m.dryRunMigrations(ctx, mm, chCurrentFile); err != nil {
		return fmt.Errorf("dry run migrations failed: %w", err)
	}

	return nil
}

// dryRunMigrations runs and rolls back migrations
func (m *Migrator) dryRunMigrations(ctx context.Context, mm Migrations, chCurrentFile chan string) (err error) {
	var tx *pg.Tx
	tx, err = m.db.Begin()
	if err != nil {
		return fmt.Errorf(`begin transaction failed: %w`, err)
	}

	defer func() {
		err = alwaysRollbackTx(tx, err)
	}()

	// apply migrations
	for _, mg := range mm {
		chCurrentFile <- mg.Filename

		// run
		start := time.Now()
		if _, err = tx.ExecContext(ctx, string(mg.Data)); err != nil {
			return fmt.Errorf(`apply migration "%s" failed: %w`, mg.Filename, err)
		}

		if err = writeMigrationToDB(ctx, mg, tx, start); err != nil {
			return err
		}
	}

	return nil
}

// Skip marks migrations as completed
func (m *Migrator) Skip(ctx context.Context, filenames []string, chCurrentFile chan string) error {
	defer close(chCurrentFile)

	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return err
	}

	// prepare migrations
	mm, err := m.newMigrations(filenames)
	if err != nil {
		return fmt.Errorf("prepare migrations failed: %w", err)
	}

	// skip migrations
	if err := m.skipMigrations(ctx, mm, chCurrentFile); err != nil {
		return fmt.Errorf("skip migrations failed: %w", err)
	}
	return nil
}

func (m *Migrator) skipMigrations(ctx context.Context, mm Migrations, chCurrentFile chan string) (err error) {
	var tx *pg.Tx
	tx, err = m.db.Begin()
	if err != nil {
		return fmt.Errorf(`begin transaction failed: %w`, err)
	}

	defer func() {
		err = finishTxOnErr(tx, err)
	}()

	// write migrations to pgMigrations table
	for _, mg := range mm {
		chCurrentFile <- mg.Filename
		if err = writeMigrationToDB(ctx, mg, tx, time.Now()); err != nil {
			return err
		}
	}

	return nil
}

func alwaysRollbackTx(tx *pg.Tx, err error) error {
	if er := tx.Rollback(); er != nil {
		err = fmt.Errorf("failed to finish transaction er=%w: %w", er, err)
	}

	return err
}

// Last shows applied migrations
func (m *Migrator) Last(ctx context.Context, num int) ([]PgMigration, error) {
	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return nil, err
	}

	// fetch last migrations
	var pm []PgMigration
	if err := m.db.ModelContext(ctx, &pm).Order(`id DESC`).Limit(num).Select(); err != nil {
		return nil, fmt.Errorf(`fetch last %d migrations failed: %w`, num, err)
	}

	return pm, nil
}

// Verify compare md5 sum applied migrations with migrations in filesystem.
// It returns invalid migrations by md5sum.
func (m *Migrator) Verify(ctx context.Context) ([]PgMigration, error) {
	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return nil, err
	}

	// read all Files
	filenames, err := m.readAllFiles()
	if err != nil {
		return nil, err
	}

	// create migrations
	mm, err := m.newMigrations(filenames)
	if err != nil {
		return nil, fmt.Errorf("prepare migrations failed: %w", err)
	}

	// fetch completed migrations from db
	var pm []PgMigration
	if err = m.db.ModelContext(ctx, &pm).Where(`"filename" in (?)`, pg.In(filenames)).Select(); err != nil {
		return nil, fmt.Errorf("fetch completed migrations failed: %w", err)
	}

	return m.compareMD5Sum(mm.ToDB(), pm), nil
}

// compareMD5Sum compare md5 sum completed migrations with files in root dir
func (m *Migrator) compareMD5Sum(all, completed []PgMigration) (res []PgMigration) {
	allMapping := make(map[string]string)
	for _, n := range all {
		allMapping[n.Filename] = n.Md5sum
	}

	for _, f := range completed {
		if sum := allMapping[f.Filename]; sum != f.Md5sum {
			f.Md5sumLocal = sum
			res = append(res, f)
		}
	}

	return
}

// Redo rerun last migration
func (m *Migrator) Redo(ctx context.Context, chCurrentFile chan string) (*PgMigration, error) {
	// create migration table if not exists
	if err := m.createMigratorTable(ctx); err != nil {
		return nil, err
	}

	// fetch last migration
	var pm PgMigration
	if err := m.db.ModelContext(ctx, &pm).Order(`id desc`).Limit(1).Select(); err != nil {
		if errors.Is(err, pg.ErrNoRows) {
			return nil, errors.New(`applied migrations were not found`)
		}
		return nil, fmt.Errorf(`fetch last migration failed: %w`, err)
	}

	// check if migration file exists
	if _, err := os.Stat(filepath.Join(m.rootDir, pm.Filename)); err != nil {
		return nil, fmt.Errorf(`find migration file "%s" failed: %w`, pm.Filename, err)
	}

	// delete last migration from DB
	if _, err := m.db.ModelContext(ctx, &pm).WherePK().Delete(); err != nil {
		return nil, fmt.Errorf(`delete "%s" from db failed: %w`, pm.Filename, err)
	}

	// Run(filename)
	return &pm, m.Run(ctx, []string{pm.Filename}, chCurrentFile)
}

// createMigratorTable create if not exists migration table
func (m *Migrator) createMigratorTable(ctx context.Context) error {
	_, err := m.db.ExecContext(ctx, `
		create table if not exists ?
			(
				id            serial                    not null,
				filename      text                      not null,
				"startedAt"   timestamptz default now() not null,
				"finishedAt"  timestamptz,
				transactional bool        default true  not null,
				md5sum        varchar(32)               not null,
				primary key ("id"),
				unique ("filename")
			)
	`, pg.Ident(m.cfg.Table))

	return err
}
