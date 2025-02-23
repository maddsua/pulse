package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

	_ "embed"
)

type Storage interface {
	Push(entry PulseEntry) error
	QueryRange(from time.Time, to time.Time) ([]PulseEntry, error)
	Close() error
}

type ServiceStatus int

const (
	ServiceStatusUp   = 1
	ServiceStatusDown = 0
)

func (this ServiceStatus) String() string {
	switch this {
	case ServiceStatusUp:
		return "up"
	case ServiceStatusDown:
		return "down"
	default:
		return ""
	}
}

type PulseEntry struct {
	ID      sql.NullInt64
	Time    time.Time
	Label   string
	Status  ServiceStatus
	Elapsed time.Duration
}

//go:embed storage/schema.sql
var sqliteSchema string

//go:embed storage/insertSeries.sql
var sqliteQueryInsertSeries string

//go:embed storage/querySeriesRange.sql
var sqliteQuerySeriesRange string

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

	if _, err := db.Exec(sqliteSchema); err != nil {
		return nil, fmt.Errorf("failed to init schema: %s", err.Error())
	}

	return &sqliteStorage{db: db}, nil
}

type sqliteStorage struct {
	db *sql.DB
}

func (this *sqliteStorage) Close() error {
	return this.db.Close()
}

func (this *sqliteStorage) Push(entry PulseEntry) error {
	_, err := this.db.Exec(sqliteQueryInsertSeries,
		entry.Time.UnixNano(),
		entry.Label,
		entry.Status,
		entry.Elapsed)
	return err
}

func (this *sqliteStorage) QueryRange(from time.Time, to time.Time) ([]PulseEntry, error) {

	rows, err := this.db.Query(sqliteQuerySeriesRange, from.UnixNano(), to.UnixNano())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var entries []PulseEntry

	for rows.Next() {

		var next PulseEntry
		var col_time int64

		if err := rows.Scan(next.ID, col_time, next.Label, next.Status, next.Elapsed); err != nil {
			return nil, err
		}

		next.Time = time.Unix(0, col_time)

		entries = append(entries, next)
	}

	return entries, nil
}
