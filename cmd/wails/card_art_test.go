package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fetchCDNCardArt：CDN 200 → data URL 且落盘 pics/；404 → 尝试下一个 URL；
// 全部失败 → ""。用 httptest 模拟图床，不触真实网络。
func TestFetchCDNCardArt(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cards/89631139.jpg":
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(png)
		case "/cards_small/89631139.jpg":
			t.Errorf("高清命中时不应回退小图: %s", r.URL.Path)
		case "/cards/404.jpg", "/cards_small/404.jpg":
			w.WriteHeader(http.StatusNotFound)
		case "/cards_small/7.jpg":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldURLs := cardArtCDNURLs
	oldDir := cardArtCacheDir
	defer func() { cardArtCDNURLs, cardArtCacheDir = oldURLs, oldDir }()
	cardArtCDNURLs = []string{
		server.URL + "/cards/%d.jpg",
		server.URL + "/cards_small/%d.jpg",
	}
	cardArtCacheDir = t.TempDir()

	t.Run("高清命中并落盘缓存", func(t *testing.T) {
		got := fetchCDNCardArt(89631139)
		if !strings.HasPrefix(got, "data:image/jpeg;base64,") {
			t.Fatalf("期望 jpeg data URL， got %q", got[:min(len(got), 40)])
		}
		cached, err := os.ReadFile(filepath.Join(cardArtCacheDir, "89631139.jpg"))
		if err != nil || string(cached) != string(png) {
			t.Fatalf("缓存文件不符: err=%v len=%d", err, len(cached))
		}
	})

	t.Run("高清404回退小图", func(t *testing.T) {
		got := fetchCDNCardArt(7)
		if !strings.HasPrefix(got, "data:image/png;base64,") {
			t.Fatalf("期望 png data URL（小图回退），got %q", got[:min(len(got), 40)])
		}
	})

	t.Run("全部404返回空", func(t *testing.T) {
		if got := fetchCDNCardArt(404); got != "" {
			t.Fatalf("期望空字符串，got %q", got[:min(len(got), 40)])
		}
	})
}

// 同一卡号并发拉取不应触发重复下载（锁生效）。
func TestFetchCDNCardArtConcurrentSingleFlight(t *testing.T) {
	var calls int
	var mu sync.Mutex
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls++
		mu.Unlock()
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpg"))
	}))
	defer server.Close()

	oldURLs, oldDir := cardArtCDNURLs, cardArtCacheDir
	defer func() { cardArtCDNURLs, cardArtCacheDir = oldURLs, oldDir }()
	cardArtCDNURLs = []string{server.URL + "/cards/%d.jpg"}
	cardArtCacheDir = t.TempDir()

	const n = 8
	results := make(chan string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- fetchCDNCardArt(55144522)
		}()
	}
	wg.Wait()
	close(results)
	for got := range results {
		if !strings.HasPrefix(got, "data:image/jpeg") {
			t.Fatalf("并发拉取结果异常: %q", got[:min(len(got), 40)])
		}
	}
	if calls != 1 {
		t.Fatalf("期望单次下载，实际 %d 次", calls)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
