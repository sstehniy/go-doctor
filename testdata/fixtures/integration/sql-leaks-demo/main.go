package main

import "database/sql"

func run(db *sql.DB) error {
	rows, err := db.Query("SELECT id FROM users")
	if err != nil {
		return err
	}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return err
		}
	}
	return nil
}
