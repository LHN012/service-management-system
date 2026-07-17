//go:build !linux

package main

func withRootLock(root string, action func() error) error {
	return action()
}
