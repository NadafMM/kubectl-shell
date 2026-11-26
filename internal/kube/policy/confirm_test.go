package policy

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestReadAnswerYes(t *testing.T) {
	chunks := []string{"y", "e", "s", " ", "l", "e", "f", "t", "\n", "g", "e", "t"}
	if !ReadAnswer(&chunks) {
		t.Fatal("expected yes")
	}
	if strings.Join(chunks, "") != "get" {
		t.Fatal(chunks)
	}
}

func TestReadAnswerNo(t *testing.T) {
	chunks := []string{"n", "\n"}
	if ReadAnswer(&chunks) {
		t.Fatal("expected no")
	}
}

func TestAllowYes(t *testing.T) {
	if AllowYes(true, 0, true) {
		t.Fatal("empty tty shell")
	}
	if !AllowYes(true, 0, false) {
		t.Fatal("piped empty")
	}
	if !AllowYes(true, 2, true) {
		t.Fatal("command with yes")
	}
	if AllowYes(false, 0, true) {
		t.Fatal("no flag")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, errors.New("EIO") }

func TestConfirmReaderWarns(t *testing.T) {
	var warnings []string
	if ConfirmReader(errReader{}, func(text string) { warnings = append(warnings, text) }) {
		t.Fatal("expected refuse")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "EIO") {
		t.Fatal(warnings)
	}
}

func TestConfirmLine(t *testing.T) {
	var out bytes.Buffer
	if !ConfirmLine(&out, strings.NewReader("y\n"), "Run it? [y/N] ") {
		t.Fatal("yes")
	}
	if out.String() != "Run it? [y/N] " {
		t.Fatal(out.String())
	}
}
