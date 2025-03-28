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

	dec := gob.NewDecoder(f)
	err = dec.Decode(s)
	if err != nil {
		return fmt.Errorf("failed to decode file: %v", err)
	}

	return nil
}

func (db *FileDB) Save(s *selections.Set) error {
	os.MkdirAll(filepath.Dir(db.Path), 0755)

	f, err := os.Create(db.Path)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer f.Close()

	enc := gob.NewEncoder(f)
	err = enc.Encode(s)
	if err != nil {
		return err
	}
	return nil
}
