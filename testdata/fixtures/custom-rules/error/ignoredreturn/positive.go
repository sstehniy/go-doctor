package ignoredreturn

import "os"

func Positive() {
	file, _ := os.CreateTemp("", "go-doctor")
	_ = os.Remove(file.Name())
	file.Close() // want error/ignored-return
}
