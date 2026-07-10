package main

import (
	"fmt"
	"os"
	"path/filepath"

	"sms/internal/cli"
)

func main() {
	root, args, err := resolveRoot(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	application := cli.New(root, os.Stdin, os.Stdout, os.Stderr)
	if err := application.Run(args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func resolveRoot(args []string) (string, []string, error) {
	root := os.Getenv("SMS_ROOT")
	cleaned := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--root" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("--root requires a path")
			}
			root = args[i+1]
			i++
			continue
		}
		cleaned = append(cleaned, args[i])
	}
	if root == "" {
		executable, err := os.Executable()
		if err != nil {
			return "", nil, err
		}
		dir := filepath.Dir(executable)
		if filepath.Base(dir) == "bin" {
			root = filepath.Dir(dir)
		} else {
			root, err = os.Getwd()
			if err != nil {
				return "", nil, err
			}
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", nil, err
	}
	return abs, cleaned, nil
}
