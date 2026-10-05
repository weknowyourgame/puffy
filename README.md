# puffer

A search engine for vectors that keeps all its data on object storage (S3), in the style of puffy. Written in Go, standard library plus the AWS SDK, as a way to get good at programming again.

## How it works

Each piece exists to fix a problem with the piece before it:

| Problem | Fix | Name |
| --- | --- | --- |
| Data is lost on restart, and RAM is expensive | Keep the data on S3 | object storage |
| Rewriting one big file for every write is too slow | Only ever add small new files | WAL (write-ahead log) |
| Comparing the query against every vector is too slow | A background job groups similar vectors together | indexer + index |
| Knowing which index is finished, and seeing new writes | One small pointer file | manifest |
| S3 takes ~100 ms per request | Keep copies in RAM and on local SSD | cache + router (not built yet) |

Every file is written once and never edited. That one rule is what keeps the rest of the design simple, and it is also what makes the manifest safe: publishing a new version is "create the next file", and a create that finds the file already there fails, on disk (`O_EXCL`) and on S3 (`If-None-Match: *`). Two indexers racing for the same version, exactly one wins.

```
 write ──> WAL file ──> memory (recent writes)
                                              ╲
 puffer index ──> index/<build>/ ──> manifest ──> query = index clusters + recent writes
```

- **Write:** the WAL file is saved first, then the in-memory DB is updated. Writes are batched (group commit): many requests, one file.
- **Index:** `puffer index` replays the WAL, runs k-means, writes `clusters.bin` + `centroids.bin` into a new folder, then publishes a manifest pointing at it. A crash in between leaves unused files and nothing else.
- **Query:** reads the manifest, loads the centroids, and for each query only reads the `nprobe` closest clusters from the store (one range read each). The recent writes the index doesn't cover are brute forced and merged in.

## Numbers

siftsmall (10,000 vectors, 128 dims, L2), top-10, k = 100 clusters. Brute force is 1.0 recall at ~1.4 ms per query.

| nprobe | recall@10 | time per query |
| --- | --- | --- |
| 1 | 0.5800 | 35 µs |
| 5 | 0.9050 | 83 µs |
| 10 | 0.9730 | 142 µs |
| 20 | 0.9960 | 252 µs |
| 50 | 1.0000 | 531 µs |

More in [docs/nprobe-table.md](docs/nprobe-table.md). On S3, one query with `nprobe=3` makes 3 store calls.

## Layout

```
cmd/puffer/        the server, and `puffer index`
internal/db/       vectors in memory, brute force search
internal/wal/      the write-ahead log: format, writer, replay
internal/store/    where files go: a folder, S3, or a slow wrapper for testing
internal/index/    k-means, index files, cluster search
internal/manifest/ the pointer file
scratchpad/        throwaway experiments (recall, nprobe table)
docs/              file formats and notes
```

Not built yet: RAM + SSD cache, filters, BM25, sharding.

## Running

Start the server (128-dim vectors, port 8080, files in `./data`):

```bash
go run ./cmd/puffer
```

Build an index from the WAL, then restart the server so it picks up the new manifest:

```bash
go run ./cmd/puffer index
go run ./cmd/puffer -nprobe 10
```

### On S3

Any S3 compatible server works. Point puffer at a bucket with environment variables:

```bash
export AWS_ACCESS_KEY_ID=...  AWS_SECRET_ACCESS_KEY=...
export PUFFER_BUCKET=puffer
export PUFFER_S3_ENDPOINT=http://localhost:9000   # leave out for real AWS
go run ./cmd/puffer
```

Add `PUFFER_LATENCY_MS=80` to make every store call take 80 ms (like a real S3 round trip). The server then prints how many store calls each query made.

### Tests

```bash
go test ./...

# the store tests also run against S3 when a bucket is given
PUFFER_TEST_BUCKET=puffer PUFFER_S3_ENDPOINT=http://localhost:9000 go test ./internal/store
```

### curl examples

Write two vectors (128 floats, here all 0.1) and query the top 2:

```bash
VECTOR=$(python3 -c "import json; print(json.dumps([0.1]*128))")

curl -s -X POST http://localhost:8080/write \
  -H 'Content-Type: application/json' \
  -d "{\"id\": \"doc-1\", \"vector\": $VECTOR}"

curl -s -X POST http://localhost:8080/write \
  -H 'Content-Type: application/json' \
  -d "{\"id\": \"doc-2\", \"vector\": $VECTOR}"

curl -s -X POST http://localhost:8080/query \
  -H 'Content-Type: application/json' \
  -d "{\"vector\": $VECTOR, \"k\": 2}"

curl -s -X POST http://localhost:8080/delete \
  -H 'Content-Type: application/json' \
  -d '{"id": "doc-1"}'
```
