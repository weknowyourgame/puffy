# Manifest

The one pointer file. It says which index is live. Everything else is written once and never edited.

## Layout

One file per version: `manifest/<version>.json`, version zero padded to 20 digits (same as the wal).
The newest manifest is the file with the highest version. Nothing is ever overwritten.

```json
{
  "version": 3,
  "index": "00000000000000000120-1791227607184955000",
  "index_seq": 120,
  "dim": 128,
  "k": 100
}
```

- `version`: the file name. 1, 2, 3...
- `index`: the live index is `index/<index>/`. Every build gets its own folder (wal seq + build time), so two indexers or a retry after a crash never collide.
- `index_seq`: the wal sequence the index covers, up to and including.
- `dim`, `k`: vector size and number of clusters in that index.

## Publishing (compare-and-swap)

1. Indexer reads the latest manifest (version N, or nothing).
2. Builds the index and writes both index files. Nobody points at them yet.
3. Creates `manifest/<N+1>.json`. Create fails if the file exists (`O_EXCL` on disk, `If-None-Match: *` on S3).

Two indexers both read N, both build, both try to create N+1. One Create wins, the other gets
`ErrConflict` and backs off. The loser's index files are just unused garbage in `index/`.

## Why it is safe

- Crash before step 3: no new manifest, queries keep using the old index.
- Queries only open the index the manifest points at, never one that is half written.
- The version number in the file name plays the part of the ETag.

## Why JSON here and not for vectors

It is tiny, read once at startup, and wants to be readable by a human. Vectors are millions of
floats: JSON would be ~3x bigger and slow to parse, and floats don't survive text without care.
