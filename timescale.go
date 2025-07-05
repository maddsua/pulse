package pulse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type timescaleMigrator func(ctx context.Context, db *sql.DB) error

var timescaleTableDefs = map[string]timescaleMigrator{
	"pulse_uptime_v2": func(ctx context.Context, db *sql.DB) error {

		_, err := db.ExecContext(ctx, `create table if not exists pulse_uptime_v2 (
			time timestamp with time zone not null,
			label text not null,
			probe_elapsed int8 not null,
			probe_type text not null,
			up boolean not null,
			latency int8,
			host text,
			http_status int2,
			tls_version int2
		)`)

		return err
	},
}

func NewTimescaleStorage(dbUrl string) (*timescaleStorage, error) {

	const version = "v2"

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, err
	} else if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}

	tables := map[string]string{
		"uptime": "pulse_uptime_" + version,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var tableExists = func(table string) (bool, error) {

		_, err := db.QueryContext(ctx, fmt.Sprintf("select exists (select 1 from %s)", table))
		if err == nil || strings.Contains(err.Error(), "does not exist") {
			return err == nil, nil
		}

		return false, err
	}

	for key, table := range tables {

		migrator := timescaleTableDefs[table]
		if migrator == nil {
			return nil, fmt.Errorf("table migration not found for '%s' (%s)", key, table)
		}

		if exists, _ := tableExists(table); !exists {

			slog.Info("TIMESCALE: Setting up",
				slog.String("table", table))

			if err := migrator(ctx, db); err != nil {
				return nil, fmt.Errorf("table migration failed for '%s' (%s): %v", key, table, err)
			}
		}
	}

	return &timescaleStorage{
		db:      db,
		version: version,
		tables:  tables,
	}, nil
}

type timescaleStorage struct {
	db      *sql.DB
	version string
	tables  map[string]string
}

func (this *timescaleStorage) Type() string {
	return "timescale"
}

func (this *timescaleStorage) Version() string {
	return this.version
}

func (this *timescaleStorage) Close() error {
	return this.db.Close()
}

func (this *timescaleStorage) insertContext(ctx context.Context, tablename string, row map[string]any) error {

	var columns []string
	var args []any
	for col, val := range row {
		columns = append(columns, col)
		args = append(args, val)
	}

	var bindvars []string
	for idx := range columns {
		bindvars = append(bindvars, "$"+strconv.Itoa(idx+1))
	}

	query := fmt.Sprintf("insert into %s (%s) values (%s)",
		tablename,
		strings.Join(columns, ", "),
		strings.Join(bindvars, ", "))

	_, err := this.db.ExecContext(ctx, query, args...)
	return err
}

func (this *timescaleStorage) WriteUptime(ctx context.Context, entry UptimeEntry) error {

	tablename, has := this.tables["uptime"]
	if !has {
		return errors.New("unable to find target table")
	}

	if entry.Label == "" {
		return errors.New("empty entry label")
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	if entry.ProbeType == "" {
		entry.ProbeType = "generic"
	}

	row := map[string]any{
		"label":         entry.Label,
		"time":          entry.Timestamp,
		"up":            entry.Up,
		"probe_type":    entry.ProbeType,
		"probe_elapsed": entry.ProbeElapsed.Milliseconds(),
	}

	if entry.Latency != nil {
		row["latency"] = entry.Latency.Milliseconds()
	}

	if entry.HttpStatus != nil {
		row["http_status"] = *entry.HttpStatus
	}

	if entry.Host != nil {
		row["host"] = *entry.Host
	}

	if entry.TlsVersion != nil {
		row["tls_version"] = *entry.TlsVersion
	}

	return this.insertContext(ctx, tablename, row)
}
