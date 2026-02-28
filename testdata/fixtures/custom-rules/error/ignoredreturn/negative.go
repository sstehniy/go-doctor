package ignoredreturn

import "os"

func Negative() error {
	file, err := os.CreateTemp("", "go-doctor")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	return file.Close()
}
