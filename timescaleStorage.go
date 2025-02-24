package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/guregu/null"
	_ "github.com/lib/pq"
	"github.com/maddsua/pulse/storage/timescale/queries"
	_ "github.com/mattn/go-sqlite3"

	"github.com/golang-migrate/migrate/v4"
	postgres_migrate "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed storage/timescale/migrations/*
var timescaleMigfs embed.FS

func NewTimescaleStorage(dbUrl string) (*timescaleStorage, error) {

	connUrl, err := url.Parse(dbUrl)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, err
	}

	slog.Debug("Storage: Timescale enabled",
		slog.String("host", connUrl.Host),
		slog.String("name", strings.TrimPrefix(connUrl.Path, "/")))

	storage := &timescaleStorage{db: db, queries: queries.New(db)}

	if err := storage.migrate(db); err != nil {
		return nil, fmt.Errorf("failed to run storage migrations: %s", err.Error())
	}

	return storage, nil
}

type timescaleStorage struct {
	db      *sql.DB
	queries *queries.Queries
}

func (this *timescaleStorage) Close() error {
	return this.db.Close()
}

func (this *timescaleStorage) Push(entry PulseEntry) error {
	return this.queries.InsertSeries(context.Background(), queries.InsertSeriesParams{
		Time:       entry.Time,
		Label:      entry.Label,
		Status:     entry.Status.String(),
		HttpStatus: sql.NullInt16{Int16: int16(entry.HttpStatus.Int64), Valid: entry.HttpStatus.Valid},
		Elapsed:    entry.Elapsed.Milliseconds(),
	})
}

func (this *timescaleStorage) QueryRange(from time.Time, to time.Time) ([]PulseEntry, error) {

	entries, err := this.queries.GetSeriesRange(context.Background(), queries.GetSeriesRangeParams{
		RangeFrom: from,
		RangeTo:   to,
	})
	if err != nil {
		return nil, err
	}

	result := make([]PulseEntry, len(entries))
	for idx, val := range entries {
		result[idx] = PulseEntry{
			ID:         null.IntFrom(val.ID),
			Time:       val.Time,
			Label:      val.Label,
			Status:     ParseServiceStatus(val.Status),
			HttpStatus: null.NewInt(int64(val.HttpStatus.Int16), val.HttpStatus.Valid),
			Elapsed:    time.Duration(val.Elapsed) * time.Millisecond,
		}
	}

	return result, nil
}

func (this *timescaleStorage) migrate(db *sql.DB) error {

	migfs, err := iofs.New(timescaleMigfs, "storage/timescale/migrations")
	if err != nil {
		return err
	}

	migdb, err := postgres_migrate.WithInstance(db, &postgres_migrate.Config{})
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithInstance("iofs", migfs, "postgresql", migdb)
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
