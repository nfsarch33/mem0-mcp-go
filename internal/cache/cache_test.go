package cache

import (
	"sync"
	"testing"
	"time"
)

func TestGet_MissOnEmptyCache(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 100, TTL: time.Minute})

	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected miss on empty cache")
	}
}

func TestSetGet_ReturnsHit(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 100, TTL: time.Minute})

	data := []byte(`{"results":[]}`)
	c.Set("query1", data)

	got, ok := c.Get("query1")
	if !ok {
		t.Fatal("expected hit after Set")
	}
	if string(got) != string(data) {
		t.Fatalf("got %q, want %q", got, data)
	}
}

func TestTTLExpiry_ReturnsMiss(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 100, TTL: 50 * time.Millisecond})

	c.Set("expiring", []byte("data"))

	time.Sleep(80 * time.Millisecond)

	_, ok := c.Get("expiring")
	if ok {
		t.Fatal("expected miss after TTL expiry")
	}
}

func TestLRUEviction_WhenMaxExceeded(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 3, TTL: time.Minute})

	c.Set("a", []byte("1"))
	c.Set("b", []byte("2"))
	c.Set("c", []byte("3"))

	// Access "a" to make it recently used
	c.Get("a")

	// Add a 4th entry — should evict "b" (least recently used)
	c.Set("d", []byte("4"))

	if _, ok := c.Get("b"); ok {
		t.Fatal("expected 'b' to be evicted (LRU)")
	}
	if _, ok := c.Get("a"); !ok {
		t.Fatal("expected 'a' to survive (recently accessed)")
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("expected 'c' to survive")
	}
	if _, ok := c.Get("d"); !ok {
		t.Fatal("expected 'd' to survive (just added)")
	}
}

func TestInvalidate_RemovesEntry(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 100, TTL: time.Minute})

	c.Set("key1", []byte("value1"))
	c.Invalidate("key1")

	_, ok := c.Get("key1")
	if ok {
		t.Fatal("expected miss after Invalidate")
	}
}

func TestStats_TracksHitsMissesEvictions(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 2, TTL: time.Minute})

	// Miss
	c.Get("x")

	// Set + Hit
	c.Set("a", []byte("1"))
	c.Get("a")

	// Fill to capacity then evict
	c.Set("b", []byte("2"))
	c.Set("c", []byte("3")) // evicts "a" (LRU — "b" was Set last but "a" wasn't touched since Get)

	stats := c.Stats()
	if stats.Hits != 1 {
		t.Fatalf("hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Fatalf("misses = %d, want 1", stats.Misses)
	}
	if stats.Evictions != 1 {
		t.Fatalf("evictions = %d, want 1", stats.Evictions)
	}
}

func TestConcurrentGetSet_IsSafe(t *testing.T) {
	t.Parallel()
	c := New(Options{MaxEntries: 100, TTL: time.Minute})

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		key := string(rune('A' + i%26))
		go func() {
			defer wg.Done()
			c.Set(key, []byte("value"))
		}()
		go func() {
			defer wg.Done()
			c.Get(key)
		}()
	}
	wg.Wait()
}
