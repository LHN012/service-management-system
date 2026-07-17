//go:build !linux

package main

import (
	// fmt 用来返回说明性错误。
	"fmt"
	// os 提供终端文件句柄类型。
	"os"
)

type terminalState struct{}

// isTerminal 在非 Linux 环境先不启用方向键交互。
func isTerminal(file *os.File) bool {
	return false
}

// enterRawMode 在非 Linux 环境不会被正常调用。
func enterRawMode(file *os.File) (*terminalState, error) {
	return nil, fmt.Errorf("raw terminal mode is only implemented for linux")
}

// restoreTerminal 在非 Linux 环境不需要做事。
func restoreTerminal(file *os.File, state *terminalState) {}
