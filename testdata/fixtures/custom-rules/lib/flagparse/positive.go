package flagparse

import "flag"

func Positive() {
	flag.Parse() // want lib/flag-parse-in-non-main
}
