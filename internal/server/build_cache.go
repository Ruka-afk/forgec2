package server

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type CacheStats struct {
	Hits   int64
	Misses int64
	Size   int64
	Count  int
}

type buildCacheEntry struct {
	Path      string
	Size      int64
	CreatedAt time.Time
}

type BuildCache struct {
	cacheDir string
	maxSize  int64
	ttl      time.Duration
	mu       sync.RWMutex
	entries  map[string]*buildCacheEntry
	hits     int64
	misses   int64
	stopCh   chan struct{}
}

func NewBuildCache(cacheDir string, maxSizeMB int, ttl time.Duration) (*BuildCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}

	bc := &BuildCache{
		cacheDir: cacheDir,
		maxSize:  int64(maxSizeMB) * 1024 * 1024,
		ttl:      ttl,
		entries:  make(map[string]*buildCacheEntry),
		stopCh:   make(chan struct{}),
	}

	bc.loadExisting()
	bc.Cleanup()

	return bc, nil
}

func (bc *BuildCache) loadExisting() {
	_ = filepath.Walk(bc.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		name := info.Name()
		ext := filepath.Ext(name)
		hash := name[:len(name)-len(ext)]
		if len(hash) != 64 {
			return nil
		}
		bc.entries[hash] = &buildCacheEntry{
			Path:      path,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		}
		return nil
	})
}

func (bc *BuildCache) Lookup(paramsHash string) (string, bool) {
	bc.mu.RLock()
	entry, ok := bc.entries[paramsHash]
	bc.mu.RUnlock()

	if !ok {
		bc.mu.Lock()
		bc.misses++
		bc.mu.Unlock()
		return "", false
	}

	if time.Since(entry.CreatedAt) > bc.ttl {
		bc.mu.Lock()
		bc.misses++
		os.Remove(entry.Path)
		delete(bc.entries, paramsHash)
		bc.mu.Unlock()
		return "", false
	}

	bc.mu.Lock()
	bc.hits++
	bc.mu.Unlock()
	return entry.Path, true
}

func (bc *BuildCache) Store(paramsHash string, data []byte, ext string) (string, error) {
	if ext != "" && ext[0] != '.' {
		ext = "." + ext
	}
	fileName := paramsHash + ext
	path := filepath.Join(bc.cacheDir, fileName)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write cache file: %w", err)
	}

	bc.mu.Lock()
	bc.entries[paramsHash] = &buildCacheEntry{
		Path:      path,
		Size:      int64(len(data)),
		CreatedAt: time.Now(),
	}
	bc.mu.Unlock()

	return path, nil
}

func (bc *BuildCache) Cleanup() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	now := time.Now()
	var totalSize int64
	expired := make([]string, 0)

	for hash, entry := range bc.entries {
		if now.Sub(entry.CreatedAt) > bc.ttl {
			expired = append(expired, hash)
			os.Remove(entry.Path)
			continue
		}
		totalSize += entry.Size
	}

	for _, hash := range expired {
		delete(bc.entries, hash)
	}

	if totalSize <= bc.maxSize {
		return
	}

	type kv struct {
		hash string
		at   time.Time
	}
	var byAge []kv
	for hash, entry := range bc.entries {
		byAge = append(byAge, kv{hash: hash, at: entry.CreatedAt})
	}
	for i := 0; i < len(byAge); i++ {
		for j := i + 1; j < len(byAge); j++ {
			if byAge[j].at.Before(byAge[i].at) {
				byAge[i], byAge[j] = byAge[j], byAge[i]
			}
		}
	}

	for _, item := range byAge {
		if totalSize <= bc.maxSize {
			break
		}
		entry, exists := bc.entries[item.hash]
		if !exists {
			continue
		}
		totalSize -= entry.Size
		os.Remove(entry.Path)
		delete(bc.entries, item.hash)
	}
}

func (bc *BuildCache) StartCleanup(ctx context.Context, interval time.Duration) {
	bc.stopCh = make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bc.Cleanup()
			case <-ctx.Done():
				bc.Cleanup()
				return
			case <-bc.stopCh:
				return
			}
		}
	}()
}

func (bc *BuildCache) Stop() {
	select {
	case bc.stopCh <- struct{}{}:
	default:
	}
}

func (bc *BuildCache) Stats() CacheStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	var totalSize int64
	for _, entry := range bc.entries {
		totalSize += entry.Size
	}

	return CacheStats{
		Hits:   bc.hits,
		Misses: bc.misses,
		Size:   totalSize,
		Count:  len(bc.entries),
	}
}

func HashParams(params interface{}) string {
	data, err := json.Marshal(params)
	if err != nil {
		slog.Error("Failed to marshal params for cache hash", "error", err)
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}
