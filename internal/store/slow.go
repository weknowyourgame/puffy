package store

import (
	"sync/atomic"
	"time"
)

// Slow wraps a Store and adds a delay to every call, like a real S3 round trip (~80 ms).
// It also counts the calls, so we can see how many round trips a query makes.
type Slow struct {
	inner Store
	delay time.Duration
	calls atomic.Int64
}

func NewSlow(s Store, delay time.Duration) *Slow {
	return &Slow{inner: s, delay: delay}
}

// Calls is how many store calls have been made so far.
func (s *Slow) Calls() int64 { return s.calls.Load() }

func (s *Slow) wait() {
	s.calls.Add(1)
	time.Sleep(s.delay)
}

func (s *Slow) Read(name string) ([]byte, error) {
	s.wait()
	return s.inner.Read(name)
}

func (s *Slow) ReadAt(name string, off int64, n int) ([]byte, error) {
	s.wait()
	return s.inner.ReadAt(name, off, n)
}

func (s *Slow) Create(name string, b []byte) error {
	s.wait()
	return s.inner.Create(name, b)
}

func (s *Slow) List(p Prefix) ([]string, error) {
	s.wait()
	return s.inner.List(p)
}
