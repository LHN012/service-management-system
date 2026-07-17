package audit

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
	"time"
)

type Entry struct {
	Time          time.Time `json:"time"`
	User          string    `json:"user"`
	Command       string    `json:"command"`
	Target        string    `json:"target,omitempty"`
	Action        string    `json:"action"`
	Confirmations int       `json:"confirmations,omitempty"`
	Result        string    `json:"result"`
	Error         string    `json:"error,omitempty"`
}

func Write(root string, entry Entry) error {
	entry.Time = time.Now()
	if current, err := user.Current(); err == nil {
		entry.User = current.Username
	}
	path := filepath.Join(root, "data", "logs", "audit.log")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(entry)
}
