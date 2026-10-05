package store

import "errors"

// Create returns ErrExists when the name is already taken. Nothing is ever overwritten,
// which is also what makes the manifest safe: two indexers racing for the same
// manifest version, exactly one Create succeeds.
var ErrExists = errors.New("already exists")

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
	// ReadAt reads n bytes starting at offset off, without loading the whole file
	ReadAt(s string, off int64, n int) (b []byte, e error)
	// Create writes a new file and never overwrites: ErrExists if it is already there
	Create(n string, b []byte) (e error)
	List(p Prefix) (n []string, e error)
}
