package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

// Same tests for every Store. Add an implementation = add one line to a Test func below.
func testStore(t *testing.T, s Store) {
	// each run gets its own prefix so a bucket that was used before doesn't matter
	p := fmt.Sprintf("t%d/", time.Now().UnixNano())

	// Create + Read
	if err := s.Create(p+"b.bin", []byte("hello world")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read(p + "b.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("hello world")) {
		t.Fatalf("read %q", got)
	}

	// Create never overwrites
	err = s.Create(p+"b.bin", []byte("other"))
	if !errors.Is(err, ErrExists) {
		t.Fatalf("second create: want ErrExists, got %v", err)
	}
	got, _ = s.Read(p + "b.bin")
	if string(got) != "hello world" {
		t.Fatalf("file was overwritten: %q", got)
	}

	// ReadAt gets just a slice
	got, err = s.ReadAt(p+"b.bin", 6, 5)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "world" {
		t.Fatalf("readat %q", got)
	}

	// List: sorted, only the prefix
	s.Create(p+"c.bin", []byte("c"))
	s.Create(p+"a.bin", []byte("a"))
	s.Create(p+"sub/d.bin", []byte("d"))
	names, err := s.List(Prefix(p))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{p + "a.bin", p + "b.bin", p + "c.bin", p + "sub/d.bin"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("list: got %v, want %v", names, want)
	}

	// a prefix with nothing in it is just empty
	names, err = s.List(Prefix(p + "nothing/"))
	if err != nil || len(names) != 0 {
		t.Fatalf("empty list: %v %v", names, err)
	}
}

func TestFileStore(t *testing.T) {
	testStore(t, NewStore(t.TempDir()))
}

func TestSlowStore(t *testing.T) {
	s := NewSlow(NewStore(t.TempDir()), time.Millisecond)
	testStore(t, s)
	if s.Calls() == 0 {
		t.Fatal("calls were not counted")
	}
}

// Needs MinIO: docker compose up -d, create the bucket, then
// PUFFY_TEST_BUCKET=puffy PUFFY_S3_ENDPOINT=http://localhost:9000 AWS_ACCESS_KEY_ID=puffy AWS_SECRET_ACCESS_KEY=puffy-secret go test ./internal/store
func TestS3Store(t *testing.T) {
	bucket := os.Getenv("PUFFY_TEST_BUCKET")
	if bucket == "" {
		t.Skip("PUFFY_TEST_BUCKET not set")
	}
	s, err := NewS3Store(bucket, os.Getenv("PUFFY_S3_ENDPOINT"))
	if err != nil {
		t.Fatal(err)
	}
	testStore(t, s)
}
