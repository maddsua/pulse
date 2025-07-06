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

func NewTimescaleStorage(dbUrl string) (*timescaleStorage, error) {

	const version = "v2"

	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		return nil, err
	}

	tableName := "pulse_uptime_" + version

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	var tableExists = func() (bool, error) {

		query := fmt.Sprintf("select exists (select 1 from %s)", tableName)

		_, err := db.QueryContext(ctx, query)
		if err == nil || strings.Contains(err.Error(), "does not exist") {
			return err == nil, nil
		}

		return false, err
	}

	var tableInit = func() error {

		query := fmt.Sprintf(`create table %s (
			time timestamp with time zone not null,
			label text not null,
			probe_elapsed int8 not null,
			probe_type text not null,
			up boolean not null,
			latency int8,
			host text,
			http_status int2,
			tls_version int2
		)`, tableName)

		_, err := db.ExecContext(ctx, query)
		return err
	}

	if exists, _ := tableExists(); !exists {

		slog.Info("TIMESCALE: Setting up",
			slog.String("table", tableName))

		if err := tableInit(); err != nil {
			db.Close()
			return nil, err
		}
	}

	return &timescaleStorage{
		db:      db,
		version: version,
		table:   tableName,
	}, nil
}

type timescaleStorage struct {
	db      *sql.DB
	version string
	table   string
}

// Returns client TypeID
func (this *timescaleStorage) Type() string {
	return "timescale"
}

// Returns client version
func (this *timescaleStorage) Version() string {
	return this.version
}

// Closes the database connections
func (this *timescaleStorage) Close() error {
	return this.db.Close()
}

func (this *timescaleStorage) Ping() error {
	return this.db.Ping()
}

// Writes a single uptime metric
func (this *timescaleStorage) WriteUptime(ctx context.Context, entry UptimeEntry) error {

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

	return sqlInsertContext(ctx, this.db, this.table, row)
}

func sqlInsertContext(ctx context.Context, db *sql.DB, table string, row map[string]any) error {

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
		table,
		strings.Join(columns, ", "),
		strings.Join(bindvars, ", "))

	_, err := db.ExecContext(ctx, query, args...)
	return err
}
