package osexit

import "os"

func Positive() {
	os.Exit(1) // want lib/os-exit-in-non-main
}
