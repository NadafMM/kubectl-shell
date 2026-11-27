//go:build !windows

package kube

import (
	"io"
	"os"
)

// OpenTTYInput opens the controlling terminal for reading input.
func OpenTTYInput() (*os.File, func(), error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

// OpenTTYPrompt opens the controlling terminal for interactive confirmation prompts.
func OpenTTYPrompt(_ io.Writer) (io.Reader, io.Writer, func(), error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, nil, func() {}, err
	}
	return f, f, func() { _ = f.Close() }, nil
}
