package sqlite

import (
	"database/sql"

	"github.com/guregu/null"
)

func WrapNullTime(val null.Time) sql.NullInt64 {

	if !val.Valid {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: val.Time.UnixNano(), Valid: true}
}
