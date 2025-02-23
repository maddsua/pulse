package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maddsua/pulse/storage/sqlite/queries"
	_ "github.com/mattn/go-sqlite3"

	"github.com/golang-migrate/migrate/v4"
	sqlite_migrate "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed storage/sqlite/migrations/*
var sqliteMigfs embed.FS

func NewSqliteStorage(path string) (*sqliteStorage, error) {

	params := url.Values{}

	if before, after, has := strings.Cut(path, "?"); has {

		query, err := url.ParseQuery(after)
		if err != nil {
			return nil, err
		}

		path = before
		params = query
	}

	params.Set("_journal", "WAL")

	switch {
	case strings.HasSuffix(path, ".db"), strings.HasSuffix(path, ".db3"):
	default:
		path = filepath.Join(path, "./storage.db3")
	}

	if dir := filepath.Dir(path); dir != "." && dir != "/" && dir != "\\" {
		if _, err := os.Stat(dir); err != nil {
			if err := os.Mkdir(dir, os.ModePerm); err != nil {
				return nil, err
			}
		}
	}

	if len(params) > 0 {
		path = path + "?" + params.Encode()
	}

	slog.Debug("Storage: sqlite3 enabled",
		slog.String("path", path))

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	storage := &sqliteStorage{db: db, queries: queries.New(db)}

	if err := storage.migrate(db); err != nil {
		return nil, fmt.Errorf("failed to run storage migrations: %s", err.Error())
	}

	return storage, nil
}

type sqliteStorage struct {
	db      *sql.DB
	queries *queries.Queries
}

func (this *sqliteStorage) Close() error {
	return this.db.Close()
}

func (this *sqliteStorage) Push(entry PulseEntry) error {
	return this.queries.InsertSeries(context.Background(), queries.InsertSeriesParams{
		Time:    entry.Time.UnixNano(),
		Label:   entry.Label,
		Status:  int64(entry.Status),
		Elapsed: int64(entry.Elapsed),
	})
}

func (this *sqliteStorage) QueryRange(from time.Time, to time.Time) ([]PulseEntry, error) {

	entries, err := this.queries.GetSeriesRange(context.Background(), queries.GetSeriesRangeParams{
		RangeFrom: from.UnixNano(),
		RangeTo:   to.UnixNano(),
	})
	if err != nil {
		return nil, err
	}

	result := make([]PulseEntry, len(entries))
	for idx, val := range entries {
		result[idx] = PulseEntry{
			ID:      sql.NullInt64{Int64: val.ID, Valid: true},
			Time:    time.Unix(0, val.Time),
			Label:   val.Label,
			Status:  ServiceStatus(val.Status),
			Elapsed: time.Duration(val.Elapsed),
		}
	}

	return result, nil
}

func (this *sqliteStorage) migrate(db *sql.DB) error {

	migfs, err := iofs.New(sqliteMigfs, "storage/sqlite/migrations")
	if err != nil {
		return err
	}

	migdb, err := sqlite_migrate.WithInstance(db, &sqlite_migrate.Config{})
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithInstance("iofs", migfs, "sqlite3", migdb)
	if err != nil {
		return err
	}

	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	version, ditry, err := mig.Version()
	if err != nil {
		return err
	}

	slog.Debug("Storage migrated",
		slog.Int("version", int(version)),
		slog.Bool("dirty", ditry))

	return nil
}
