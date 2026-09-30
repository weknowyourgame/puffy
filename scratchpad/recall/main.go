package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/weknowyourgame/puffer/internal/db"
)

func main() {
	base, dim, _, err := LoadFvecs("../../testdata/siftsmall/siftsmall_base.fvecs")
	if err != nil {
		fmt.Println("load base:", err)
		return
	}
	queries, _, _, err := LoadFvecs("../../testdata/siftsmall/siftsmall_query.fvecs")
	if err != nil {
		fmt.Println("load queries:", err)
		return
	}
	gt, gtDim, _, err := LoadIvecs("../../testdata/siftsmall/siftsmall_groundtruth.ivecs")
	if err != nil {
		fmt.Println("load ground truth:", err)
		return
	}

	d := db.New(128)

	// Load 10,000 base vectors into the DB
	for i := 0; i < 10000; i++ {
		d.Upsert(db.Vector{Key: strconv.Itoa(i), Values: base[i*dim : (i+1)*dim]})
	}

	// Query and measure recall@10
	hits := 0
	for q := 0; q < 100; q++ {
		qv := queries[q*dim : (q+1)*dim]
		results, err := d.Query(db.Vector{Values: qv}, 10)
		if err != nil {
			fmt.Println("query error:", err)
			return
		}

		truth := make(map[string]bool, 10)
		for _, id := range gt[q*gtDim : q*gtDim+10] {
			truth[strconv.Itoa(int(id))] = true
		}

		for _, r := range results {
			if truth[r.Key] {
				hits++
			}
		}
	}

	recall := float64(hits) / float64(100*10)
	fmt.Printf("recall@10 = %.4f\n", recall)
}

func LoadFvecs(filepath string) (base []float32, dim int, nbase int, err error) {
	file, err := os.ReadFile(filepath)
	if err != nil {
		return nil, 0, 0, err
	}

	offset := 0
	for offset < len(file) {
		d := int(binary.LittleEndian.Uint32(file[offset : offset+4]))
		offset += 4

		for i := 0; i < d; i++ {
			bits := binary.LittleEndian.Uint32(file[offset : offset+4])
			base = append(base, math.Float32frombits(bits))
			offset += 4
		}

		dim = d
		nbase++
	}

	return base, dim, nbase, nil
}

// LoadIvecs reads a .ivecs file: each vector is a 4-byte dim header
// followed by dim int32 values. Used for ground-truth neighbor IDs.
func LoadIvecs(filepath string) (base []int32, dim int, nbase int, err error) {
	file, err := os.ReadFile(filepath)
	if err != nil {
		return nil, 0, 0, err
	}

	offset := 0
	for offset < len(file) {
		d := int(binary.LittleEndian.Uint32(file[offset : offset+4]))
		offset += 4

		for i := 0; i < d; i++ {
			bits := binary.LittleEndian.Uint32(file[offset : offset+4])
			base = append(base, int32(bits))
			offset += 4
		}

		dim = d
		nbase++
	}

	return base, dim, nbase, nil
}
