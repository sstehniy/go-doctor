package txrollback

import "database/sql"

func begin() (*sql.Tx, error) { return nil, nil }

func Positive() {
	tx, _ := begin() // want db/tx-no-deferred-rollback
	_ = tx
}

func Conditional(err error) error {
	tx, _ := begin() // want db/tx-no-deferred-rollback
	if err != nil {
		tx.Rollback()
		return err
	}
	return nil
}
