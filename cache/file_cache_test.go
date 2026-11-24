package cache

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type testData struct {
	Header string
}

func TestFileCache_BasicOperations(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := NewFileCache[testData](tmpDir, 10*time.Minute, ctx)
	defer fc.Destroy()

	key := CacheKey{Hex: "testkey1"}
	content := []byte("hello world reservoir")

	// 1. Initial Get should miss
	_, err := fc.Get(key)
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss, got %v", err)
	}

	// 2. Cache entry
	entry, err := fc.Cache(key, bytes.NewReader(content), time.Now().Add(1*time.Hour), testData{Header: "v1"})
	if err != nil {
		t.Fatalf("failed to cache: %v", err)
	}
	defer entry.Data.Close()

	// 3. Get should hit
	hit, err := fc.Get(key)
	if err != nil {
		t.Fatalf("expected cache hit, got %v", err)
	}
	defer hit.Data.Close()

	data, err := io.ReadAll(hit.Data)
	if err != nil {
		t.Fatalf("failed to read data: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Fatalf("expected %s, got %s", content, data)
	}
}

func TestFileCache_EmptyFileCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := NewFileCache[testData](tmpDir, 10*time.Minute, ctx)
	defer fc.Destroy()

	key := CacheKey{Hex: "emptykey"}
	// Write empty content (0 bytes)
	_, err := fc.Cache(key, bytes.NewReader([]byte{}), time.Now().Add(1*time.Hour), testData{})
	if err == nil {
		t.Fatalf("expected error for empty cache file, got nil")
	}

	// Verify file is removed from disk and not leaked
	filePath := filepath.Join(tmpDir, key.Hex)
	if _, statErr := os.Stat(filePath); !os.IsNotExist(statErr) {
		t.Fatalf("expected empty file to be removed, but it exists")
	}
}

func TestFileCache_ConcurrentAccess_RaceSafe(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := NewFileCache[testData](tmpDir, 10*time.Minute, ctx)
	defer fc.Destroy()

	key := CacheKey{Hex: "concurrencykey"}
	content := []byte("concurrency payload data")

	entry, err := fc.Cache(key, bytes.NewReader(content), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache: %v", err)
	}
	entry.Data.Close()

	var wg sync.WaitGroup
	// Spin up 20 concurrent goroutines reading and fetching metadata
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				e, err := fc.Get(key)
				if err == nil {
					e.Data.Close()
				}
				_, _, _ = fc.GetMetadata(key)
			}
		}()
	}
	wg.Wait()
}

func TestFileCache_PurgeAll_And_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := NewFileCache[testData](tmpDir, 10*time.Minute, ctx)
	defer fc.Destroy()

	key1 := CacheKey{Hex: "purgekey1"}
	key2 := CacheKey{Hex: "purgekey2"}

	// Cache two entries
	e1, err := fc.Cache(key1, bytes.NewReader([]byte("data1")), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache key1: %v", err)
	}
	e1.Data.Close()

	e2, err := fc.Cache(key2, bytes.NewReader([]byte("data2")), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache key2: %v", err)
	}
	e2.Data.Close()

	// 1. Delete single key
	deleted, err := fc.Delete(key1)
	if err != nil || !deleted {
		t.Fatalf("expected key1 to be deleted, got deleted=%v, err=%v", deleted, err)
	}

	// Verify key1 is gone
	_, err = fc.Get(key1)
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss for key1, got %v", err)
	}

	// Deleting again should return false (not found)
	deletedAgain, err := fc.Delete(key1)
	if err != nil || deletedAgain {
		t.Fatalf("expected deletedAgain=false, got %v, err=%v", deletedAgain, err)
	}

	// 2. PurgeAll should remove key2 and any remaining disk files
	count, err := fc.PurgeAll()
	if err != nil {
		t.Fatalf("PurgeAll failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 entry purged, got %d", count)
	}

	// Verify key2 is also gone
	_, err = fc.Get(key2)
	if err != ErrCacheMiss {
		t.Fatalf("expected ErrCacheMiss for key2, got %v", err)
	}
}

func TestFileCache_MemoryCache(t *testing.T) {
	tmpDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fc := NewFileCache[testData](tmpDir, 10*time.Minute, ctx)
	defer fc.Destroy()

	// 1. When max memory size is 0: memory caching is disabled
	fc.SetMaxMemCacheSize(0)
	key0 := CacheKey{Hex: "mem0"}
	content0 := []byte("disk only content")
	e0, err := fc.Cache(key0, bytes.NewReader(content0), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache: %v", err)
	}
	e0.Data.Close()

	if fc.GetMemByteSize() != 0 {
		t.Fatalf("expected 0 memory usage when disabled, got %d", fc.GetMemByteSize())
	}

	hit0, err := fc.Get(key0)
	if err != nil {
		t.Fatalf("failed to get: %v", err)
	}
	hit0.Data.Close()

	// 2. Enable memory cache with 500 bytes
	fc.SetMaxMemCacheSize(500)
	key1 := CacheKey{Hex: "mem1"}
	content1 := []byte("hello memory cached world")
	e1, err := fc.Cache(key1, bytes.NewReader(content1), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache: %v", err)
	}
	e1.Data.Close()

	if fc.GetMemByteSize() != int64(len(content1)) {
		t.Fatalf("expected memory byte size %d, got %d", len(content1), fc.GetMemByteSize())
	}

	// Read should hit memory
	hit1, err := fc.Get(key1)
	if err != nil {
		t.Fatalf("failed to get from memory cache: %v", err)
	}
	defer hit1.Data.Close()

	data1, err := io.ReadAll(hit1.Data)
	if err != nil || !bytes.Equal(data1, content1) {
		t.Fatalf("data mismatch from memory cache: %s", data1)
	}

	// 3. Test LRU Eviction: small limit (50 bytes)
	fc.SetMaxMemCacheSize(50)
	key2 := CacheKey{Hex: "mem2"}
	content2 := bytes.Repeat([]byte("A"), 30)
	e2, err := fc.Cache(key2, bytes.NewReader(content2), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache key2: %v", err)
	}
	e2.Data.Close()

	key3 := CacheKey{Hex: "mem3"}
	content3 := bytes.Repeat([]byte("B"), 30)
	e3, err := fc.Cache(key3, bytes.NewReader(content3), time.Now().Add(1*time.Hour), testData{})
	if err != nil {
		t.Fatalf("failed to cache key3: %v", err)
	}
	e3.Data.Close()

	// Memory usage should not exceed limit
	if fc.GetMemByteSize() > 50 {
		t.Fatalf("memory usage %d exceeded limit 50", fc.GetMemByteSize())
	}

	// key2 might be evicted from memory, but MUST still be readable from disk!
	hit2, err := fc.Get(key2)
	if err != nil {
		t.Fatalf("failed to get key2 after memory eviction: %v", err)
	}
	defer hit2.Data.Close()
	data2, _ := io.ReadAll(hit2.Data)
	if !bytes.Equal(data2, content2) {
		t.Fatalf("corrupted content for key2")
	}
}
