package rowserr

type DB struct{}
type Rows struct{}

func (DB) Query() (*Rows, error) { return &Rows{}, nil }
func (*Rows) Close() error       { return nil }
func (*Rows) Next() bool         { return false }
func (*Rows) Err() error         { return nil }

func Negative(db DB) {
	rows, _ := db.Query()
	defer rows.Close()
	for rows.Next() {
	}
	_ = rows.Err()
}
