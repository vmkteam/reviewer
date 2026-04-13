package migrator

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Table            string
	StatementTimeout string
	FileMask         string
}

func NewDefaultConfig() Config {
	return Config{
		Table:            "public.pgMigrations",
		StatementTimeout: "5s",
		FileMask:         `\d{4}-\d{2}-\d{2}-\S+.sql`,
	}
}

type PgMigration struct {
	tableName struct{} `pg:"?migrationTable,alias:t,discard_unknown_columns"` //nolint:all

	ID            int        `pg:"id,pk"`
	Filename      string     `pg:"filename,use_zero"`
	StartedAt     time.Time  `pg:"startedAt,use_zero"`
	FinishedAt    *time.Time `pg:"finishedAt"`
	Transactional bool       `pg:"transactional,use_zero"`
	Md5sum        string     `pg:"md5sum,use_zero"`
	Md5sumLocal   string     `pg:"-"`
}

type Migration struct {
	Filename      string
	Data          []byte
	Md5Sum        string
	Transactional bool
}

func NewMigration(rootDir, filename string) (Migration, error) {
	f, err := os.ReadFile(filepath.Join(rootDir, filename))
	if err != nil {
		return Migration{Filename: filename}, err
	}

	m := Migration{
		Filename:      filename,
		Data:          f,
		Md5Sum:        fmt.Sprintf("%x", md5.Sum(f)),
		Transactional: !strings.HasSuffix(filename, "NONTR.sql"),
	}

	return m, nil
}

func (m *Migration) ToDB() *PgMigration {
	return &PgMigration{
		Filename:      m.Filename,
		Transactional: m.Transactional,
		Md5sum:        m.Md5Sum,
	}
}

type Migrations []Migration

func (mm Migrations) FirstNonTransactional() (*Migration, bool) {
	for _, m := range mm {
		if !m.Transactional {
			return &m, true
		}
	}

	return nil, false
}

func (mm Migrations) ToDB() []PgMigration {
	if len(mm) == 0 {
		return nil
	}

	out := make([]PgMigration, 0, len(mm))
	for _, m := range mm {
		out = append(out, *m.ToDB())
	}

	return out
}
