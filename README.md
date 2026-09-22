# puffer

A search engine for vectors that keeps all its data on object storage (S3), in the style of [turbopuffer](https://turbopuffer.com/docs/architecture). Written in Go, **by hand**, as a way to get good at programming again.

> This is a learning project. The code is written by me, not generated. AI tools working in this repo follow the rules in [AGENTS.md](AGENTS.md): they teach, they don't write the code.

## How it works (the short version)

Each piece exists to fix a problem with the piece before it:

| Problem | Fix | Name |
| --- | --- | --- |
| Data is lost on restart, and RAM is expensive | Keep the data on S3 | object storage |
| Rewriting one big file for every write is too slow | Only ever add small new files | WAL (write-ahead log) |
| Comparing the query against every vector is too slow | A background job groups similar vectors together | indexer + index |
| Knowing which index is finished, and seeing new writes | One small pointer file | manifest |
| S3 takes ~100 ms per request | Keep copies in RAM and on local SSD | cache + router |

Only the manifest ever changes. Every other file is written once and never edited, and that one rule is what keeps the rest of the design simple.

## Status

Current milestone: **M0 — setup**. See [ROADMAP.md](ROADMAP.md) and the task files in [milestones/](milestones/).

## Layout (grows one milestone at a time)

```
cmd/puffer/        the server binary            (M0)
internal/db/       vectors in memory, search    (M0–M1)
internal/store/    where the files go (disk, then S3)  (M2)
internal/wal/      the write-ahead log          (M2)
internal/index/    clustering + index files     (M4)
internal/manifest/ the pointer file             (M5)
internal/cache/    RAM + SSD cache              (M7)
docs/              notes, journal, decisions
```

Don't create a folder until the milestone that needs it.

## Running

Nothing to run yet. You'll add the commands here once they exist.
