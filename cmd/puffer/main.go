package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/weknowyourgame/puffer/internal/db"
)

func main() {
	mydb := db.New(db.Dim128)

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
		if err := mydb.Upsert(db.Vector{Key: req.ID, Values: req.Vector}); err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
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
		results, err := mydb.Query(db.Vector{Values: req.Vector}, req.K)
		if err != nil {
			respond(w, 400, map[string]string{"error": err.Error()})
			return
		}
		respond(w, 200, map[string]any{"results": results})
	})

	addr := ":8080"
	fmt.Println("puffer listening on", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body)
}
