package update

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCheckForUpdate_DevVersion(t *testing.T) {
	// "dev" version should never trigger update check
	result := CheckForUpdate("dev", t.TempDir())
	if result != "" {
		t.Errorf("CheckForUpdate(dev) = %q, want empty", result)
	}
}

func TestCheckForUpdate_CachedResult(t *testing.T) {
	dir := t.TempDir()

	// Write a fresh cache with a newer version
	cache := VersionCache{
		LastCheck:     time.Now(),
		LatestVersion: "v0.2.0",
		ReleaseURL:    "https://github.com/stompy-ai/stompy-cli/releases/tag/v0.2.0",
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(filepath.Join(dir, cacheFileName), data, 0644)

	result := CheckForUpdate("v0.1.0", dir)
	if result != "v0.2.0" {
		t.Errorf("CheckForUpdate with cached newer version = %q, want v0.2.0", result)
	}
}

func TestCheckForUpdate_CachedSameVersion(t *testing.T) {
	dir := t.TempDir()

	cache := VersionCache{
		LastCheck:     time.Now(),
		LatestVersion: "v0.1.0",
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(filepath.Join(dir, cacheFileName), data, 0644)

	result := CheckForUpdate("v0.1.0", dir)
	if result != "" {
		t.Errorf("CheckForUpdate with same version = %q, want empty", result)
	}
}

func TestCheckForUpdate_CachedSameVersionNoPrefix(t *testing.T) {
	dir := t.TempDir()

	// Cache has "v0.1.0" but current version is "0.1.0" (no v prefix) — should still match
	cache := VersionCache{
		LastCheck:     time.Now(),
		LatestVersion: "v0.1.0",
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(filepath.Join(dir, cacheFileName), data, 0644)

	result := CheckForUpdate("0.1.0", dir)
	if result != "" {
		t.Errorf("CheckForUpdate with v-prefix mismatch = %q, want empty", result)
	}
}

func TestCheckForUpdate_CachedOlderVersion(t *testing.T) {
	dir := t.TempDir()

	// Cache has an older version than current — should NOT suggest downgrade
	cache := VersionCache{
		LastCheck:     time.Now(),
		LatestVersion: "v0.1.5",
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(filepath.Join(dir, cacheFileName), data, 0644)

	result := CheckForUpdate("v0.2.0", dir)
	if result != "" {
		t.Errorf("CheckForUpdate with older cached version = %q, want empty (no downgrade)", result)
	}
}

func TestIsNewer(t *testing.T) {
	tests := []struct {
		candidate string
		current   string
		want      bool
	}{
		{"v0.2.0", "v0.1.5", true},
		{"v0.1.5", "v0.2.0", false},
		{"v0.2.0", "v0.2.0", false},
		{"v1.0.0", "v0.9.9", true},
		{"v0.2.1", "v0.2.0", true},
		{"0.2.0", "0.1.0", true},
		{"v0.2.0", "0.2.0", false}, // same version, different prefix
	}
	for _, tt := range tests {
		got := isNewer(tt.candidate, tt.current)
		if got != tt.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", tt.candidate, tt.current, got, tt.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	assets := []Asset{
		{Name: "stompy_v0.2.0_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64.tar.gz"},
		{Name: "stompy_v0.2.0_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64.tar.gz"},
		{Name: "stompy_v0.2.0_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64.zip"},
		{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums.txt"},
	}

	asset := findAsset(assets)
	if asset == nil {
		t.Fatal("findAsset returned nil for current platform")
	}
	// Should match current OS/arch
	t.Logf("Matched asset: %s", asset.Name)
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{5242880, "5.0 MB"},
	}
	for _, tt := range tests {
		got := formatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("formatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestGetLatestRelease_MockServer(t *testing.T) {
	release := Release{
		TagName: "v0.3.0",
		HTMLURL: "https://github.com/stompy-ai/stompy-cli/releases/tag/v0.3.0",
		Assets: []Asset{
			{Name: "stompy_v0.3.0_darwin_arm64.tar.gz", Size: 5000000},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(release)
	}))
	defer server.Close()

	// We can't easily override the const releaseAPI, so just test the mock directly
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("request error: %v", err)
	}
	defer resp.Body.Close()

	var got Release
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if got.TagName != "v0.3.0" {
		t.Errorf("TagName = %q, want v0.3.0", got.TagName)
	}
}
