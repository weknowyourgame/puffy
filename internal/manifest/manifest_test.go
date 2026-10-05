package manifest

import (
	"errors"
	"sync"
	"testing"

	"github.com/weknowyourgame/puffer/internal/store"
)

func TestNoManifest(t *testing.T) {
	_, err := Latest(store.NewStore(t.TempDir()))
	if !errors.Is(err, ErrNoManifest) {
		t.Fatalf("want ErrNoManifest, got %v", err)
	}
}

func TestPublishAndLatest(t *testing.T) {
	s := store.NewStore(t.TempDir())
	for v := uint64(1); v <= 3; v++ {
		if err := Publish(s, Manifest{Version: v, Index: "x", IndexSeq: v * 10, Dim: 128, K: 5}); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Latest(s)
	if err != nil {
		t.Fatal(err)
	}
	if m.Version != 3 || m.IndexSeq != 30 || m.Dim != 128 || m.K != 5 {
		t.Fatalf("got %+v", m)
	}
}

// Two indexers read the same manifest, both try to publish the next version.
func TestRace(t *testing.T) {
	s := store.NewStore(t.TempDir())
	Publish(s, Manifest{Version: 1, Index: "x", IndexSeq: 1, Dim: 128, K: 1})

	const racers = 20
	errs := make([]error, racers)
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = Publish(s, Manifest{Version: 2, IndexSeq: uint64(100 + i), Dim: 128, K: 1})
		}()
	}
	wg.Wait()

	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatalf("a loser got %v, want ErrConflict", err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d winners, want exactly 1", wins)
	}

	// and what is live is the winner's
	m, _ := Latest(s)
	if m.Version != 2 {
		t.Fatalf("latest is version %d", m.Version)
	}
}
