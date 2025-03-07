// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0
// source: queries.sql

package queries

import (
	"context"
	"database/sql"
)

const getTlsSeriesRange = `-- name: GetTlsSeriesRange :many
select id, time, label, security, cert_subject, cert_issuer, cert_expires, cert_fingerprint from tlscert
where time >= ?1
	and time <= ?2
`

type GetTlsSeriesRangeParams struct {
	RangeFrom int64
	RangeTo   int64
}

func (q *Queries) GetTlsSeriesRange(ctx context.Context, arg GetTlsSeriesRangeParams) ([]Tlscert, error) {
	rows, err := q.db.QueryContext(ctx, getTlsSeriesRange, arg.RangeFrom, arg.RangeTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Tlscert
	for rows.Next() {
		var i Tlscert
		if err := rows.Scan(
			&i.ID,
			&i.Time,
			&i.Label,
			&i.Security,
			&i.CertSubject,
			&i.CertIssuer,
			&i.CertExpires,
			&i.CertFingerprint,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getUptimeSeriesRange = `-- name: GetUptimeSeriesRange :many
select id, time, label, status, http_status, elapsed, latency from uptime
where time >= ?1
	and time <= ?2
`

type GetUptimeSeriesRangeParams struct {
	RangeFrom int64
	RangeTo   int64
}

func (q *Queries) GetUptimeSeriesRange(ctx context.Context, arg GetUptimeSeriesRangeParams) ([]Uptime, error) {
	rows, err := q.db.QueryContext(ctx, getUptimeSeriesRange, arg.RangeFrom, arg.RangeTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Uptime
	for rows.Next() {
		var i Uptime
		if err := rows.Scan(
			&i.ID,
			&i.Time,
			&i.Label,
			&i.Status,
			&i.HttpStatus,
			&i.Elapsed,
			&i.Latency,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertTls = `-- name: InsertTls :exec
insert into tlscert (
	time,
	label,
	security,
	cert_subject,
	cert_issuer,
	cert_expires,
	cert_fingerprint
) values (
	?1,
	?2,
	?3,
	?4,
	?5,
	?6,
	?7
)
`

type InsertTlsParams struct {
	Time            int64
	Label           string
	Security        string
	CertSubject     sql.NullString
	CertIssuer      sql.NullString
	CertExpires     sql.NullInt64
	CertFingerprint sql.NullString
}

func (q *Queries) InsertTls(ctx context.Context, arg InsertTlsParams) error {
	_, err := q.db.ExecContext(ctx, insertTls,
		arg.Time,
		arg.Label,
		arg.Security,
		arg.CertSubject,
		arg.CertIssuer,
		arg.CertExpires,
		arg.CertFingerprint,
	)
	return err
}

const insertUptime = `-- name: InsertUptime :exec
insert into uptime (
	time,
	label,
	status,
	http_status,
	elapsed,
	latency
) values (
	?1,
	?2,
	?3,
	?4,
	?5,
	?6
)
`

type InsertUptimeParams struct {
	Time       int64
	Label      string
	Status     string
	HttpStatus sql.NullInt64
	Elapsed    int64
	Latency    int64
}

func (q *Queries) InsertUptime(ctx context.Context, arg InsertUptimeParams) error {
	_, err := q.db.ExecContext(ctx, insertUptime,
		arg.Time,
		arg.Label,
		arg.Status,
		arg.HttpStatus,
		arg.Elapsed,
		arg.Latency,
	)
	return err
}
