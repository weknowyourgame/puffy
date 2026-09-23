package main

import (
	"fmt"
	"math/rand/v2"

	"github.com/weknowyourgame/puffer/internal/db"
)

func main() {

	// passing 128 dim, New builds a DB struct here and passes us the pointer
	mydb := db.New(db.Dim128)
	r1, _ := RandVector(db.Dim128)
	r2, _ := RandVector(db.Dim128)
	k := 10

	fmt.Println("Random vector1", r1)
	fmt.Println("Random vector1", r2)
	// Build vectors
	v1 := db.Vector{"a", r1}
	v2 := db.Vector{"b", r2}

	// create the vector
	mydb.Upsert(v1)
	mydb.Upsert(v2)

	// queries against other vectors
	q1, _ := mydb.Query(v1, k)
	q2, _ := mydb.Query(v2, k)

	fmt.Println("Query1", q1)
	fmt.Println("Query1", q2)
}

func RandVector(d db.Dimension) ([]float32, error) {
	if d != db.Dim128 && d != db.Dim256 && d != db.Dim512 {
		return nil, fmt.Errorf("wrong dimension passed: %v", d)
	}
	v := make([]float32, d)

	for i := range v {
		v[i] = float32(rand.NormFloat64())
	}
	return v, nil
}
