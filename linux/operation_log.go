package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// OperationLogEntry is one structured SMS operation record.
type OperationLogEntry struct {
	Timestamp string `json:"timestamp"`
	Action    string `json:"action"`
	Command   string `json:"command,omitempty"`
	Result    string `json:"result"`
	Detail    string `json:"detail,omitempty"`
}

// OperationLogger writes monthly JSON Lines logs and archives completed months.
type OperationLogger struct {
	root string
	now  func() time.Time
	mu   sync.Mutex
}

func newOperationLogger(root string) (*OperationLogger, error) {
	logger := &OperationLogger{root: root, now: time.Now}
	if err := os.MkdirAll(logger.archiveDir(), 0o750); err != nil {
		return nil, err
	}
	return logger, nil
}

// Log writes one entry. Archive failures are reported without skipping the new entry.
func (l *OperationLogger) Log(action string, command string, result string, detail string) error {
	if l == nil {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now().Local()
	archiveErr := l.archiveCompletedMonths(now)
	entry := OperationLogEntry{
		Timestamp: now.Format(time.RFC3339),
		Action:    action,
		Command:   command,
		Result:    result,
		Detail:    detail,
	}
	writeErr := l.writeEntry(now, entry)
	return errors.Join(archiveErr, writeErr)
}

func (l *OperationLogger) logDir() string {
	return filepath.Join(l.root, "logs")
}

func (l *OperationLogger) archiveDir() string {
	return filepath.Join(l.logDir(), "archive")
}

func (l *OperationLogger) activeLogPath(now time.Time) string {
	return filepath.Join(l.logDir(), "sms-"+now.Format("2006-01")+".log")
}

func (l *OperationLogger) writeEntry(now time.Time, entry OperationLogEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	file, err := os.OpenFile(l.activeLogPath(now), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func (l *OperationLogger) archiveCompletedMonths(now time.Time) error {
	entries, err := os.ReadDir(l.logDir())
	if err != nil {
		return err
	}

	currentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	var archiveErrors []error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		month, err := time.ParseInLocation("sms-2006-01.log", entry.Name(), now.Location())
		if err != nil || !month.Before(currentMonth) {
			continue
		}

		source := filepath.Join(l.logDir(), entry.Name())
		target, err := l.nextArchivePath(strings.TrimSuffix(entry.Name(), ".log"))
		if err != nil {
			archiveErrors = append(archiveErrors, fmt.Errorf("archive %s: %w", entry.Name(), err))
			continue
		}
		if err := archiveOperationLog(source, target); err != nil {
			archiveErrors = append(archiveErrors, fmt.Errorf("archive %s: %w", entry.Name(), err))
		}
	}
	return errors.Join(archiveErrors...)
}

func (l *OperationLogger) nextArchivePath(base string) (string, error) {
	path := filepath.Join(l.archiveDir(), base+".tar.gz")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path, nil
	} else if err != nil {
		return "", err
	}

	for part := 2; ; part++ {
		path = filepath.Join(l.archiveDir(), fmt.Sprintf("%s-part-%d.tar.gz", base, part))
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
}

func archiveOperationLog(source string, target string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()

	temp, err := os.CreateTemp(filepath.Dir(target), ".sms-log-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	completed := false
	defer func() {
		temp.Close()
		if !completed {
			_ = os.Remove(tempPath)
		}
	}()

	gzipWriter := gzip.NewWriter(temp)
	tarWriter := tar.NewWriter(gzipWriter)
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.Base(source)
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}
	if _, err := io.Copy(tarWriter, input); err != nil {
		return err
	}
	if err := input.Close(); err != nil {
		return err
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	if err := gzipWriter.Close(); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, target); err != nil {
		return err
	}
	completed = true
	return os.Remove(source)
}
