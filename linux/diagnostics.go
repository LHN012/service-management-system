package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

var version = "dev"

func printVersion(output io.Writer) {
	fmt.Fprintf(output, "SMS Linux %s\n", version)
	fmt.Fprintf(output, "Go: %s\n", runtime.Version())
}

func runDoctor(output io.Writer, root string) error {
	var failures []error
	check := func(name string, err error) {
		if err == nil {
			fmt.Fprintf(output, "[ok]   %s\n", name)
			return
		}
		fmt.Fprintf(output, "[fail] %s: %v\n", name, err)
		failures = append(failures, fmt.Errorf("%s: %w", name, err))
	}

	if runtime.GOOS == "linux" {
		check("operating system", nil)
	} else {
		check("operating system", fmt.Errorf("Linux required, current system is %s", runtime.GOOS))
	}
	if _, err := os.Stat("/proc/self/stat"); err != nil {
		check("proc filesystem", err)
	} else {
		check("proc filesystem", nil)
	}
	for _, command := range []string{"sh", "nohup", "kill", "lsof"} {
		_, err := exec.LookPath(command)
		check("command "+command, err)
	}

	temporary, err := os.CreateTemp(root, ".sms-doctor-*")
	if err == nil {
		path := temporary.Name()
		err = errors.Join(temporary.Close(), os.Remove(path))
	}
	check("SMS root writable", err)
	return errors.Join(failures...)
}
