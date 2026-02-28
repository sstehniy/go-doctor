package rowsclose

import "database/sql"

func query() (*sql.Rows, error) { return nil, nil }

func Positive() {
	rows, _ := query() // want db/rows-not-closed
	_ = rows
}
