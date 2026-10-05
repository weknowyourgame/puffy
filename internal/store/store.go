package store

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
	Create(n string, b []byte) (e error)
	List(p Prefix) (n []string, e error)
}
