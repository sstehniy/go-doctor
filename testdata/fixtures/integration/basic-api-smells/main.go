package main

import "errors"

var MutableGlobal = map[string]string{}

func isEOF(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "EOF"
}

func main() {
	_ = isEOF(errors.New("EOF"))
}
