package execinput

import (
	"os"
	"os/exec"
)

func Positive() {
	_ = exec.Command("sh", os.Getenv("USER_INPUT")) // want sec/exec-user-input
}
