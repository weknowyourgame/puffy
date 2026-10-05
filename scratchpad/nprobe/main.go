package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/weknowyourgame/puffer/internal/db"
	"github.com/weknowyourgame/puffer/internal/index"
)

func main() {
	dir := "../../testdata/siftsmall/siftsmall"
	if len(os.Args) > 1 {
		dir = os.Args[1] // e.g. ../../testdata/sift/sift
	}

	base, dim, nbase, err := LoadFvecs(dir + "_base.fvecs")
	if err != nil {
		fmt.Println("load base:", err)
		return
	}
	queries, _, nq, err := LoadFvecs(dir + "_query.fvecs")
	if err != nil {
		fmt.Println("load queries:", err)
		return
	}
	gt, gtDim, _, err := LoadIvecs(dir + "_groundtruth.ivecs")
	if err != nil {
		fmt.Println("load ground truth:", err)
		return
	}

	fmt.Println("base:", nbase, "x", dim, " queries:", nq, " ground truth:", len(gt)/gtDim, "x", gtDim)

	// recall@10 of one query: how many of its 10 results are in the true top 10
	hitsFor := func(q int, results []db.Result) int {
		truth := make(map[string]bool, 10)
		for _, id := range gt[q*gtDim : q*gtDim+10] {
			truth[strconv.Itoa(int(id))] = true
		}
		hits := 0
		for _, r := range results {
			if truth[r.Key] {
				hits++
			}
		}
		return hits
	}

	// SIFT ground truth is computed with L2
	d := db.New(db.Dimension(dim), db.L2)
	keys := make([]string, nbase)
	for i := 0; i < nbase; i++ {
		keys[i] = strconv.Itoa(i)
		d.Upsert(db.Vector{Key: keys[i], Values: base[i*dim : (i+1)*dim]})
	}

	// Brute force: should be exactly 1.0
	hits := 0
	start := time.Now()
	for q := 0; q < nq; q++ {
		results, err := d.Query(db.Vector{Values: queries[q*dim : (q+1)*dim]}, 10)
		if err != nil {
			fmt.Println("query error:", err)
			return
		}
		hits += hitsFor(q, results)
	}
	fmt.Printf("brute force: recall@10 = %.4f, %v per query\n", float64(hits)/float64(nq*10), time.Since(start)/time.Duration(nq))

	// k-means: roughly sqrt(n) clusters
	k := int(math.Sqrt(float64(nbase)))
	start = time.Now()
	idx := index.Build(keys, base, dim, k, 25)
	fmt.Printf("k-means: n=%d k=%d took %v\n\n", nbase, k, time.Since(start))

	fmt.Println("| nprobe | recall@10 | time per query |")
	fmt.Println("| --- | --- | --- |")
	for _, nprobe := range []int{1, 5, 10, 20, 50} {
		hits := 0
		start := time.Now()
		for q := 0; q < nq; q++ {
			results, err := idx.Search(queries[q*dim:(q+1)*dim], 10, nprobe, nil)
			if err != nil {
				fmt.Println("search error:", err)
				return
			}
			hits += hitsFor(q, results)
		}
		fmt.Printf("| %d | %.4f | %v |\n", nprobe, float64(hits)/float64(nq*10), time.Since(start)/time.Duration(nq))
	}
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
