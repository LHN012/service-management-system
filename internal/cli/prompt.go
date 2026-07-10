package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var errCancelled = errors.New("operation cancelled")

func (c *CLI) prompt(label, defaultValue string, required bool) (string, error) {
	for {
		if defaultValue != "" {
			fmt.Fprintf(c.Out, "%s [%s]: ", label, defaultValue)
		} else {
			fmt.Fprintf(c.Out, "%s: ", label)
		}
		line, err := c.Reader.ReadString('\n')
		if err != nil && strings.TrimSpace(line) == "" {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value == "!cancel" {
			return "", errCancelled
		}
		if value == "" {
			value = defaultValue
		}
		if required && value == "" {
			fmt.Fprintln(c.Out, "A value is required. Use !cancel to stop.")
			continue
		}
		return value, nil
	}
}

func (c *CLI) confirm(label string) (bool, error) {
	value, err := c.prompt(label+" (yes/no)", "no", true)
	if err != nil {
		return false, err
	}
	return strings.EqualFold(value, "yes") || strings.EqualFold(value, "y"), nil
}

func (c *CLI) countPrompt(label string) (int, error) {
	for {
		value, err := c.prompt(label, "0", true)
		if err != nil {
			return 0, err
		}
		value = strings.TrimPrefix(value, "!")
		count, err := strconv.Atoi(value)
		if err == nil && count >= 0 && count <= 100 {
			return count, nil
		}
		fmt.Fprintln(c.Out, "Enter a number from 0 to 100. !N is also accepted.")
	}
}

func parsePorts(value string) ([]int, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var ports []int
	for _, item := range strings.Split(value, ",") {
		port, err := strconv.Atoi(strings.TrimSpace(item))
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port %q", item)
		}
		ports = append(ports, port)
	}
	return ports, nil
}
