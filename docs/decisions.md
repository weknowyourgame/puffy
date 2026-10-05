# Decisions

## aws-sdk-go-v2 (first dependency)
S3 requests have to be signed (SigV4), and pagination, retries and the Range / If-None-Match
headers are easy to get subtly wrong by hand. Signing is not what this project is about, so the
S3 store uses `aws-sdk-go-v2` (`config` + `service/s3`). It works with any S3 compatible server
by setting an endpoint, so tests run against a local server instead of AWS.

## Manifest = one file per version, created with "never overwrite"
`manifest/<version>.json`, and publishing is Create of the next version. Same trick on disk
(`O_EXCL`) and S3 (`If-None-Match: *`), so no extra "compare and swap" method on `Store`.

## Index folder per build
`index/<wal seq>-<build time>/`. Same seq twice (two indexers, or a retry after a crash)
must not collide, and the manifest says which folder is live.
