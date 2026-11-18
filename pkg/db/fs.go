package db

import (
	"blueclip/pkg/selections"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
)

type FileDB struct {
	Path string
}

func NewFileDB(path string) (*FileDB, error) {
	// Resolve ~ to user's home directory
	if path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %v", err)
		}
		path = home + path[1:]
	}

	return &FileDB{
		Path: path,
	}, nil
}

func (db *FileDB) Load(s *selections.Set) error {
	f, err := os.Open(db.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	err = s.Decode(gob.NewDecoder(f))
	if err != nil {
		return fmt.Errorf("failed to decode file: %v", err)
	}

	return nil
}

// Save writes to a temporary file and renames it over the history, so a crash
// mid-write never leaves a truncated history that would fail to load.
func (db *FileDB) Save(s *selections.Set) error {
	dir := filepath.Dir(db.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	// CreateTemp creates the file with 0600, the history may contain secrets.
	f, err := os.CreateTemp(dir, filepath.Base(db.Path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	tmpPath := f.Name()
	defer os.Remove(tmpPath)

	if err := s.Encode(gob.NewEncoder(f)); err != nil {
		f.Close()
		return fmt.Errorf("failed to encode selections: %v", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("failed to sync file: %v", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close file: %v", err)
	}

	if err := os.Rename(tmpPath, db.Path); err != nil {
		return fmt.Errorf("failed to replace history file: %v", err)
	}
	return nil
}
