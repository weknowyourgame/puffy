package db

import "fmt"
/*
 Vector struct stores in a kv pair
 along with a string key (hash later)
 and the dimensions it will be stored in
*/
type DB struct {
	vectors map[string][]float32
	dim int
}

type Dimension int

const (
    Dim128 Dimension = 128
    Dim256 Dimension = 256
    Dim512 Dimension = 512
)

type Vector struct {
    Key    string
    Values []float32
}

func New(dim Dimension) *DB {
	return &DB {
		vectors: make(map[string][]float32),
		dim:	 int(dim),
	}
}

func (db* DB) Upsert(v Vector) error{
	if len(v.Values) != db.dim {
        return fmt.Errorf("dimension mismatch")
    }

	db.vectors[v.Key] = v.Values
    return nil
}

func (db* DB) delete(key string){
	delete(db.vectors, key)
}

func (db* DB) count() int{
	return len(db.vectors)
}