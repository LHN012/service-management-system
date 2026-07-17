//go:build linux

package main

import "testing"

func TestWithRootLockRejectsConcurrentMutation(t *testing.T) {
	root := t.TempDir()
	if err := withRootLock(root, func() error {
		if err := withRootLock(root, func() error { return nil }); err == nil {
			t.Fatal("second state-changing command acquired the lock")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
