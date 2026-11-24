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
