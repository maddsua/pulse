// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.28.0

package queries

import (
	"database/sql"
)

type Tlscert struct {
	ID              int64
	Time            int64
	Label           string
	Security        string
	CertSubject     sql.NullString
	CertIssuer      sql.NullString
	CertExpires     sql.NullInt64
	CertFingerprint sql.NullString
}

type Uptime struct {
	ID         int64
	Time       int64
	Label      string
	Status     string
	HttpStatus sql.NullInt64
	Elapsed    int64
	Latency    int64
}
