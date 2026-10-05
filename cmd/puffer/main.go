package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/weknowyourgame/puffer/internal/db"
	"github.com/weknowyourgame/puffer/internal/index"
	"github.com/weknowyourgame/puffer/internal/manifest"
	"github.com/weknowyourgame/puffer/internal/store"
	"github.com/weknowyourgame/puffer/internal/wal"
)

const dataDir = "./data"

// set when PUFFER_LATENCY_MS is on, so /query can report how many store calls it made
var slowStore *store.Slow

func main() {
	// `puffer index` builds the index files from the WAL and exits
	if len(os.Args) > 1 && os.Args[1] == "index" {
		runIndex(os.Args[2:])
		return
	}

	nprobe := flag.Int("nprobe", 10, "how many clusters a query looks inside")
	flag.Parse()

	st := openStore()

	// The index is L2 (that is what SIFT uses), so the DB holding the recent writes is too.
	mydb := db.New(db.Dim128, db.L2)

	// The manifest says which index is live. Open only that one (if there is a manifest),
	// then replay only the WAL entries it doesn't cover.
	var idx *index.Index
	var covered uint64
	m, err := manifest.Latest(st)
	if err == nil {
		idx, err = index.Open(st, m.Index)
		if err != nil {
			log.Fatal(err)
		}
		covered = idx.Seq()
		fmt.Println("manifest", m.Version, "-> index covering wal up to", covered, "with", idx.K(), "clusters")
	} else if !errors.Is(err, manifest.ErrNoManifest) {
		log.Fatal(err)
	}
	err = wal.Replay(st, covered, func(seq uint64, ops []wal.Operation) {
		for _, op := range ops {
			switch op.Type {
			case wal.Upsert:
				mydb.Upsert(db.Vector{Key: op.ID, Values: op.Values})
			case wal.Delete:
				mydb.Delete(op.ID)
			}
		}
	})
	if err != nil {
		log.Fatal(err)
	}

	writer, err := wal.NewWriter(st, mydb)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("POST /write", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID     string    `json:"id"`
			Vector []float32 `json:"vector"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ID == "" {
			respond(w, 400, map[string]string{"error": "id is required"})
			return
		}
		if len(req.Vector) != mydb.Dim() {
			respond(w, 400, map[string]string{"error": fmt.Sprintf("vector length %d, expected %d", len(req.Vector), mydb.Dim())})
			return
		}
		// the WAL file is saved first, then the DB is updated
		if err := writer.Write([]wal.Operation{{Type: wal.Upsert, ID: req.ID, Values: req.Vector}}); err != nil {
			respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]string{"id": req.ID})
	})

	http.HandleFunc("POST /delete", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.ID == "" {
			respond(w, 400, map[string]string{"error": "id is required"})
			return
		}
		if err := writer.Write([]wal.Operation{{Type: wal.Delete, ID: req.ID}}); err != nil {
			respond(w, 500, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]string{"id": req.ID})
	})

	http.HandleFunc("POST /query", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Vector []float32 `json:"vector"`
			K      int       `json:"k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respond(w, 400, map[string]string{"error": "invalid JSON"})
			return
		}
		if req.K <= 0 {
			respond(w, 400, map[string]string{"error": "k must be a positive number"})
			return
		}
		if len(req.Vector) != mydb.Dim() {
			respond(w, 400, map[string]string{"error": fmt.Sprintf("vector length %d, expected %d", len(req.Vector), mydb.Dim())})
			return
		}
		before := int64(0)
		if slowStore != nil {
			before = slowStore.Calls()
		}
		results, err := search(idx, mydb, *nprobe, req.Vector, req.K)
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		if slowStore != nil {
			fmt.Println("query made", slowStore.Calls()-before, "store calls")
		}
		respond(w, 200, map[string]any{"results": results})
	})

	addr := ":8080"
	fmt.Println("puffer listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

/*
search = the index + everything written after it.
mydb only holds the writes the index doesn't cover, so:
  - the index is searched but ignores any id that mydb has touched (updated or deleted since),
    its copy of those is out of date
  - mydb is brute forced for the newer copies
*/
func search(idx *index.Index, mydb *db.DB, nprobe int, vector []float32, k int) ([]db.Result, error) {
	results, err := mydb.Query(db.Vector{Values: vector}, k)
	if err != nil {
		return nil, err
	}
	if idx == nil {
		return results, nil
	}

	fromIndex, err := idx.Search(vector, k, nprobe, mydb.Touched)
	if err != nil {
		return nil, err
	}

	// both lists are L2 distances, smallest first
	results = append(results, fromIndex...)
	sort.Slice(results, func(i, j int) bool { return results[i].Score < results[j].Score })
	if len(results) > k {
		results = results[:k]
	}
	return results, nil
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}

// openStore picks where the files live:
//
//	PUFFER_BUCKET=name   S3 (PUFFER_S3_ENDPOINT=http://localhost:9000 for MinIO)
//	nothing set          the ./data folder
//
// PUFFER_LATENCY_MS=80 adds a delay to every call and logs the call count on exit.
func openStore() store.Store {
	var st store.Store = store.NewStore(dataDir)

	if bucket := os.Getenv("PUFFER_BUCKET"); bucket != "" {
		s3, err := store.NewS3Store(bucket, os.Getenv("PUFFER_S3_ENDPOINT"))
		if err != nil {
			log.Fatal(err)
		}
		st = s3
	}

	if ms, err := strconv.Atoi(os.Getenv("PUFFER_LATENCY_MS")); err == nil && ms > 0 {
		slow := store.NewSlow(st, time.Duration(ms)*time.Millisecond)
		slowStore = slow
		st = slow
	}
	return st
}
