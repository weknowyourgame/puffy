package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/weknowyourgame/puffy/internal/store"
)

// See docs/manifest.md. Every version is its own file: manifest/<version>.json.
// Publishing = creating the next file, and Create never overwrites, so that is the
// "replace only if unchanged": two indexers racing for version 5, one Create wins.

var (
	ErrNoManifest = errors.New("no manifest found")
	ErrConflict   = errors.New("someone else published this manifest version first")
)

type Manifest struct {
	Version  uint64 `json:"version"`   // 1, 2, 3... the file name
	Index    string `json:"index"`     // the live index lives in index/<index>/
	IndexSeq uint64 `json:"index_seq"` // and covers the wal up to and including this
	Dim      int    `json:"dim"`
	K        int    `json:"k"` // clusters in that index
}

func name(version uint64) string { return fmt.Sprintf("manifest/%020d.json", version) }

// Latest reads the newest manifest, or returns ErrNoManifest.
func Latest(s store.Store) (*Manifest, error) {
	keys, err := s.List("manifest/")
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, ErrNoManifest
	}

	// keys are sorted and zero padded, so the last one is the newest
	latest := keys[len(keys)-1]
	n, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(latest, "manifest/"), ".json"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", latest, err)
	}

	b, err := s.Read(latest)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", latest, err)
	}
	if m.Version != n {
		return nil, fmt.Errorf("%s: version inside is %d", latest, m.Version)
	}
	return &m, nil
}

// Publish makes m the live manifest. m.Version must be the latest version + 1.
// If another indexer already took that version, it returns ErrConflict and
// the caller re-reads with Latest and decides what to do.
func Publish(s store.Store, m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	err = s.Create(name(m.Version), b)
	if errors.Is(err, store.ErrExists) {
		return ErrConflict
	}
	return err
}
