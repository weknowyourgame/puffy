# Index files

An index covers the WAL up to and including sequence number `seq`. It is two files, written
once and never edited:

```
index/<seq as 20 digits>/clusters.bin
index/<seq as 20 digits>/centroids.bin
```

All integers are **little-endian**, floats are `float32` bits. Both files start with the same header.

## Header (22 bytes)

| Field   | Bytes | Notes                              |
| ------- | ----- | ---------------------------------- |
| Magic   | 4     | `pufi`                             |
| Version | 2     | format version, checked first      |
| Seq     | 8     | last WAL sequence number covered   |
| K       | 4     | number of clusters                 |
| Dim     | 4     | numbers per vector                 |

## centroids.bin

Header, then `k * dim` floats. Centroid `c` is floats `[c*dim, (c+1)*dim)`.
Size = `22 + k*dim*4`. It is small (k=1000, dim=128 is 512 KB), so it is read whole.

## clusters.bin

```
header (22)
offsets: k+1 x uint64      cluster c is the bytes [offsets[c], offsets[c+1])
cluster 0
cluster 1
...
```

Offsets are absolute positions in the file. The last offset is the end of the file.

Each cluster:

| Field   | Bytes    | Notes                                  |
| ------- | -------- | -------------------------------------- |
| Count   | 4        | uint32, vectors in this cluster        |
| Entries | variable | `count` entries, below                 |

Each entry: id length (uint16), id (UTF-8), `dim` floats.

## Reading

Open reads `centroids.bin` and the header + offset table of `clusters.bin` (`22 + 8*(k+1)` bytes).
A query ranks the centroids, then reads only the `nprobe` clusters it needs with
`ReadAt(offsets[c], offsets[c+1]-offsets[c])`. The big file is never loaded whole.

## Why each cluster is packed together

A query reads whole clusters. If a cluster is one contiguous range, that is one read of one range.
If its vectors were scattered through the file it would be one read per vector. On S3 each request
costs ~100 ms, so one range request per cluster is what makes this design work at all.

## Commit order

`clusters.bin` is written first and `centroids.bin` last. A crash in between leaves a folder
with no `centroids.bin`, and Open ignores it. (The manifest in M5 replaces this trick.)
