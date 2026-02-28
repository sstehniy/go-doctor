package rowserr

import "database/sql"

func query() (*sql.Rows, error) { return nil, nil }

func Positive() {
	rows, _ := query() // want db/rows-err-not-checked
	defer rows.Close()
	for rows.Next() {
	}
}
