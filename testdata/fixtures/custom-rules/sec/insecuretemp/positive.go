package insecuretemp

import (
	"os"
	"path/filepath"
)

func Positive() {
	file, _ := os.Create(filepath.Join(os.TempDir(), "known-name")) // want sec/insecure-temp-file
	if file != nil {
		_ = file.Close()
	}
}
