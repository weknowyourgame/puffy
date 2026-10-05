package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

type FileStore struct {
	root string
}

func NewStore(root string) *FileStore {
	return &FileStore{
		root: root,
	}
}

func (s *FileStore) Create(name string, data []byte) error {
	path := filepath.Join(s.root, name)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	/*
	* os.0_CREATE -> Create file if it doesnt exist
	* os.O_EXCL -> exclusive creation (fail if the file already exists)
	* CREATE + EXCL -> Create a new file, but never overwrite an existing one
	* os.O_WRONLY -> Open the file for write only
	 */
	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_EXCL|os.O_WRONLY,
		0644,
	)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrExists
		}
		return err
	}

	// Cleanup
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func (s *FileStore) Read(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(s.root, name))
}

// ReadAt reads exactly n bytes starting at off.
func (s *FileStore) ReadAt(name string, off int64, n int) ([]byte, error) {
	file, err := os.Open(filepath.Join(s.root, name))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	b := make([]byte, n)
	if _, err := file.ReadAt(b, off); err != nil {
		return nil, err
	}
	return b, nil
}

// List returns every file under the prefix (e.g. "wal/"), sorted by name.
// A prefix that doesn't exist yet is just empty.
func (s *FileStore) List(p Prefix) ([]string, error) {
	dir := filepath.Join(s.root, string(p))

	var names []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}
		names = append(names, filepath.ToSlash(rel))
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// WalkDir already walks in lexical order, sort anyway so the contract is explicit
	sort.Strings(names)
	return names, nil
}
