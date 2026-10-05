package wal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/weknowyourgame/puffer/internal/store"
)

// Replay decodes every WAL file with a sequence number greater than `after`,
// in order, and hands each batch of operations to apply.
// Files at or below `after` are skipped without being read.
func Replay(s store.Store, after uint64, apply func(seq uint64, ops []Operation)) error {
	keys, err := s.List("wal/")
	if err != nil {
		return err
	}

	for _, key := range keys {
		name := strings.TrimSuffix(strings.TrimPrefix(key, "wal/"), ".bin")
		n, err := strconv.ParseUint(name, 10, 64)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		if n <= after {
			continue
		}

		data, err := s.Read(key)
		if err != nil {
			return err
		}
		seq, ops, err := Decode(data)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		apply(seq, ops)
	}
	return nil
}
