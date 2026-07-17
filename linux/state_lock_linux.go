//go:build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func withRootLock(root string, action func() error) error {
	path := filepath.Join(root, ".sms.lock")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
			return fmt.Errorf("another SMS state-changing command is running")
		}
		return err
	}
	defer unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return action()
}
