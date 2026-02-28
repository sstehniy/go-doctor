package txrollback

import "database/sql"

func beginNegative() (*sql.Tx, error) { return nil, nil }

func Negative() {
	tx, _ := beginNegative()
	defer tx.Rollback()
}
