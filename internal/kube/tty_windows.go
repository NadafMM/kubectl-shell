//go:build windows

package kube

import (
	"io"
	"os"

	"golang.org/x/term"
)

// OpenTTYInput opens CONIN$ (or os.Stdin if attached to a console) on Windows.
func OpenTTYInput() (*os.File, func(), error) {
	f, err := os.OpenFile("CONIN$", os.O_RDONLY, 0)
	if err == nil {
		return f, func() { _ = f.Close() }, nil
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return os.Stdin, func() {}, nil
	}
	return nil, func() {}, err
}

// OpenTTYPrompt opens Windows console handles for interactive confirmation prompts.
func OpenTTYPrompt(fallbackOut io.Writer) (io.Reader, io.Writer, func(), error) {
	in, closeIn, err := OpenTTYInput()
	if err != nil {
		return nil, nil, func() {}, err
	}
	outFile, outErr := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if outErr == nil {
		return in, outFile, func() {
			_ = outFile.Close()
			closeIn()
		}, nil
	}
	if fallbackOut == nil {
		fallbackOut = os.Stderr
	}
	return in, fallbackOut, closeIn, nil
}
