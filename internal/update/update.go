package update

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	githubRepo    = "stompy-ai/stompy-cli"
	releaseAPI    = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
	checkInterval = 24 * time.Hour
	cacheFileName = ".version-check"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
	HTMLURL string  `json:"html_url"`
}

// Asset represents a release asset (binary archive).
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// VersionCache stores the last check result.
type VersionCache struct {
	LastCheck     time.Time `json:"last_check"`
	LatestVersion string    `json:"latest_version"`
	ReleaseURL    string    `json:"release_url"`
}

// CheckForUpdate queries GitHub for the latest release, using a 24h cache.
// Returns the latest version string (e.g. "v0.2.0") or "" if current is up to date.
func CheckForUpdate(currentVersion, configDir string) string {
	if currentVersion == "dev" {
		return ""
	}

	cache := loadCache(configDir)
	if cache != nil && time.Since(cache.LastCheck) < checkInterval {
		if cache.LatestVersion != "" && isNewer(cache.LatestVersion, currentVersion) {
			return cache.LatestVersion
		}
		return ""
	}

	// Fetch latest release from GitHub (with short timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(releaseAPI)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}

	// Save cache
	saveCache(configDir, VersionCache{
		LastCheck:     time.Now(),
		LatestVersion: release.TagName,
		ReleaseURL:    release.HTMLURL,
	})

	if isNewer(release.TagName, currentVersion) {
		return release.TagName
	}
	return ""
}

// versionsEqual compares versions ignoring the "v" prefix.
func versionsEqual(a, b string) bool {
	return strings.TrimPrefix(a, "v") == strings.TrimPrefix(b, "v")
}

// isNewer returns true if candidate is strictly newer than current (semver comparison).
func isNewer(candidate, current string) bool {
	cParts, cOk := parseSemver(candidate)
	curParts, curOk := parseSemver(current)
	if !cOk || !curOk {
		// Fall back to string inequality if we can't parse
		return !versionsEqual(candidate, current)
	}
	return compareSemver(cParts, curParts) > 0
}

// parseSemver extracts [major, minor, patch] from a version string.
func parseSemver(v string) ([3]int, bool) {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var result [3]int
	for i, p := range parts {
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return [3]int{}, false
		}
		result[i] = n
	}
	return result, true
}

// compareSemver returns -1 if a < b, 0 if equal, 1 if a > b.
func compareSemver(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// GetLatestRelease fetches the latest release info from GitHub.
func GetLatestRelease() (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(releaseAPI)
	if err != nil {
		return nil, fmt.Errorf("checking GitHub releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release info: %w", err)
	}
	return &release, nil
}

// SelfUpdate downloads and replaces the current binary with the latest release.
func SelfUpdate(currentVersion string) error {
	release, err := GetLatestRelease()
	if err != nil {
		return err
	}

	if versionsEqual(release.TagName, currentVersion) {
		return fmt.Errorf("already at latest version %s", currentVersion)
	}

	// Find the right asset for this OS/arch
	asset := findAsset(release.Assets)
	if asset == nil {
		return fmt.Errorf("no release binary found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	fmt.Printf("Downloading %s (%s)...\n", release.TagName, formatSize(asset.Size))

	// Download the archive
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get(asset.BrowserDownloadURL)
	if err != nil {
		return fmt.Errorf("downloading release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Get current executable path
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable path: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolving executable path: %w", err)
	}

	// Extract binary from archive
	var newBinary []byte
	if strings.HasSuffix(asset.Name, ".tar.gz") {
		newBinary, err = extractTarGz(resp.Body)
	} else if strings.HasSuffix(asset.Name, ".zip") {
		// For windows — download to temp and extract
		return fmt.Errorf("zip extraction not yet supported; download manually from %s", release.HTMLURL)
	} else {
		return fmt.Errorf("unknown archive format: %s", asset.Name)
	}
	if err != nil {
		return fmt.Errorf("extracting binary: %w", err)
	}

	// Write to temp file, then atomic rename
	tmpPath := execPath + ".new"
	if err := os.WriteFile(tmpPath, newBinary, 0755); err != nil {
		return fmt.Errorf("writing new binary: %w", err)
	}

	// Backup old binary
	backupPath := execPath + ".old"
	_ = os.Remove(backupPath)
	if err := os.Rename(execPath, backupPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("backing up old binary: %w", err)
	}

	if err := os.Rename(tmpPath, execPath); err != nil {
		// Rollback
		_ = os.Rename(backupPath, execPath)
		return fmt.Errorf("replacing binary: %w", err)
	}

	// Clean up backup
	_ = os.Remove(backupPath)

	fmt.Printf("Updated stompy %s → %s\n", currentVersion, release.TagName)
	return nil
}

// findAsset returns the matching asset for the current OS/arch.
func findAsset(assets []Asset) *Asset {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}

	// GoReleaser format: stompy_{version}_{os}_{arch}.tar.gz
	for i := range assets {
		name := strings.ToLower(assets[i].Name)
		if strings.Contains(name, goos) && strings.Contains(name, goarch) && strings.HasSuffix(name, ext) {
			return &assets[i]
		}
	}
	return nil
}

// extractTarGz extracts the first executable file from a tar.gz archive.
func extractTarGz(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Look for the stompy binary (not a directory, not a dotfile)
		name := filepath.Base(header.Name)
		if header.Typeflag == tar.TypeReg && (name == "stompy" || name == "stompy.exe") {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("stompy binary not found in archive")
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMG"[exp])
}

func loadCache(configDir string) *VersionCache {
	path := filepath.Join(configDir, cacheFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cache VersionCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil
	}
	return &cache
}

func saveCache(configDir string, cache VersionCache) {
	path := filepath.Join(configDir, cacheFileName)
	data, err := json.Marshal(cache)
	if err != nil {
		return
	}
	_ = os.MkdirAll(configDir, 0700)
	_ = os.WriteFile(path, data, 0644)
}
