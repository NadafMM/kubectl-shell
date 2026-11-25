package kube

import (
	"os"
	"path/filepath"
	"strconv"
)

// StateDir is ~/.kube-shell (or a temp dir when HOME is unavailable).
func StateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = os.TempDir()
	}
	return filepath.Join(home, ".kube-shell")
}

// WritePrivate creates or replaces a file with mode 0600 under a 0700 directory.
func WritePrivate(file, data string) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp := file + "." + strconv.Itoa(os.Getpid()) + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, file); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Chmod(file, 0o600)
}
