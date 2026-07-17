package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// atomicWriteFile keeps the previous file intact until the replacement is
// fully written and synced in the same directory.
func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	temporary, err := os.CreateTemp(directory, ".sms-write-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	if err := temporary.Chmod(mode); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	committed = true

	if runtime.GOOS == "linux" {
		dir, err := os.Open(directory)
		if err != nil {
			return err
		}
		syncErr := dir.Sync()
		closeErr := dir.Close()
		if syncErr != nil {
			return syncErr
		}
		return closeErr
	}
	return nil
}
