// Building blocks for s3
package store

import (
	"errors"
	"os"
	"path/filepath"
)

type Prefix string

const (
	P     Prefix = "wal"
	Index Prefix = "index"
)

const (
	Root string = "./testdata"
)

type Store interface {
	Read(s string) (b []byte, e error)
	Create(n string, b []byte) (e error)
	List(p Prefix) (n []string, e error)
}

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
			return err
		}
		return err
	}

	// Cleanup
	defer file.Close()

	_, err = file.Write(data)
	return err
}
