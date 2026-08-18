package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"certarchive/internal/domain"
)

type FileJournal struct {
	mu  sync.Mutex
	dir string
}

func NewFileJournal(dir string) (*FileJournal, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &FileJournal{dir: dir}, nil
}

func (j *FileJournal) Append(event domain.Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	checksum := checksum(data)
	record := fmt.Sprintf("%d:%s:%x\n", len(data), data, checksum)

	tmpPath := filepath.Join(j.dir, fmt.Sprintf("%d.log.tmp", time.Now().UnixNano()))
	finalPath := filepath.Join(j.dir, fmt.Sprintf("%d.log", time.Now().UnixNano()))

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(record); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	dir, err := os.Open(j.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func checksum(data []byte) byte {
	var sum byte
	for _, b := range data {
		sum ^= b
	}
	return sum
}

func (j *FileJournal) Recover() ([]domain.Event, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	files, err := os.ReadDir(j.dir)
	if err != nil {
		return nil, err
	}
	var events []domain.Event
	for _, f := range files {
		name := f.Name()
		if filepath.Ext(name) != ".log" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(j.dir, name))
		if err != nil {
			continue
		}
		parts := splitRecord(string(data))
		if len(parts) != 3 {
			continue
		}
		if verifyChecksum([]byte(parts[1]), parts[2]) {
			continue
		}
		var ev domain.Event
		if json.Unmarshal([]byte(parts[1]), &ev) == nil {
			events = append(events, ev)
		}
	}
	return events, nil
}

func splitRecord(s string) []string {
	var length int
	var idx int
	for idx < len(s) && s[idx] != ':' {
		idx++
	}
	if idx >= len(s) {
		return nil
	}
	fmt.Sscanf(s[:idx], "%d", &length)
	rest := s[idx+1:]
	if len(rest) < length+2 {
		return nil
	}
	data := rest[:length]
	if rest[length] != ':' {
		return nil
	}
	checksumStr := rest[length+1:]
	return []string{fmt.Sprintf("%d", length), data, checksumStr}
}

func verifyChecksum(data []byte, checksumStr string) bool {
	var expected byte
	fmt.Sscanf(checksumStr, "%x", &expected)
	return checksum(data) == expected
}
