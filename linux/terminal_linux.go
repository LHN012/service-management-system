//go:build linux

package main

import (
	// os 提供终端文件句柄。
	"os"

	"golang.org/x/sys/unix"
)

type terminalState struct {
	state *unix.Termios
}

// isTerminal 判断文件是不是 Linux 终端。
func isTerminal(file *os.File) bool {
	_, err := unix.IoctlGetTermios(int(file.Fd()), unix.TCGETS)
	return err == nil
}

// enterRawMode 把终端临时切到 raw mode。
// raw mode 下，方向键和空格会立刻被程序读到，不用等用户按回车。
func enterRawMode(file *os.File) (*terminalState, error) {
	fd := int(file.Fd())
	oldState, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return nil, err
	}

	newState := *oldState
	newState.Iflag &^= unix.BRKINT | unix.ICRNL | unix.INPCK | unix.ISTRIP | unix.IXON
	newState.Oflag &^= unix.OPOST
	newState.Cflag |= unix.CS8
	newState.Lflag &^= unix.ECHO | unix.ICANON | unix.IEXTEN | unix.ISIG
	newState.Cc[unix.VMIN] = 1
	newState.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &newState); err != nil {
		return nil, err
	}

	return &terminalState{state: oldState}, nil
}

// restoreTerminal 恢复 raw mode 之前的终端状态。
func restoreTerminal(file *os.File, state *terminalState) {
	if state == nil || state.state == nil {
		return
	}
	_ = unix.IoctlSetTermios(int(file.Fd()), unix.TCSETS, state.state)
}
