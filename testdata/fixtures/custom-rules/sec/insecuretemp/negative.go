package insecuretemp

import "os"

func Negative() {
	file, _ := os.CreateTemp("", "known-name")
	if file != nil {
		_ = file.Close()
	}
}
