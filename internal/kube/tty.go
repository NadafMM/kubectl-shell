package kube

import (
	"os"
	"runtime"
)

// OpenTTY opens the process controlling terminal for confirm prompts.
// On Windows this is CONIN$; elsewhere /dev/tty. Falls back to stdin when it is a TTY.
func OpenTTY() (*os.File, error) {
	name := "/dev/tty"
	if runtime.GOOS == "windows" {
		name = "CONIN$"
	}
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err == nil {
		return f, nil
	}
	if fi, statErr := os.Stdin.Stat(); statErr == nil && fi.Mode()&os.ModeCharDevice != 0 {
		return os.Stdin, nil
	}
	return nil, err
}
