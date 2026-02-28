package execinput

import "os/exec"

func Negative() {
	_ = exec.Command("sh", "-c", "echo safe")
}
