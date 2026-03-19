package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	baseURL  = "https://fantasy.premierleague.com/api"
	cacheDir = ".fpl-cache"
	cacheTTL = 1 * time.Hour
)

// Client fetches and caches FPL API data.
type Client struct {
	http     *http.Client
	cacheDir string
}

// NewClient creates an FPL API client with sensible defaults.
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: cacheDir,
	}
}

// FetchBootstrap fetches the master data: all players, teams, gameweeks.
func (c *Client) FetchBootstrap() (*BootstrapResponse, error) {
	var resp BootstrapResponse
	if err := c.fetchCached("bootstrap-static", &resp); err != nil {
		return nil, fmt.Errorf("fetching bootstrap data: %w", err)
	}
	return &resp, nil
}

// FetchFixtures fetches all fixtures for the season.
func (c *Client) FetchFixtures() ([]Fixture, error) {
	var fixtures []Fixture
	if err := c.fetchCached("fixtures", &fixtures); err != nil {
		return nil, fmt.Errorf("fetching fixtures: %w", err)
	}
	return fixtures, nil
}

// ClearCache removes all cached data, forcing fresh fetches.
func (c *Client) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}

func (c *Client) fetchCached(endpoint string, target interface{}) error {
	cacheFile := filepath.Join(c.cacheDir, endpoint+".json")

	if data, err := c.readCache(cacheFile); err == nil {
		return json.Unmarshal(data, target)
	}

	data, err := c.fetchRaw(endpoint)
	if err != nil {
		return err
	}

	// Best-effort cache write — don't fail the request if caching fails
	_ = c.writeCache(cacheFile, data)

	return json.Unmarshal(data, target)
}

func (c *Client) fetchRaw(endpoint string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/", baseURL, endpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fpl-picker/1.0")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("FPL API returned %d for %s", resp.StatusCode, endpoint)
	}

	return io.ReadAll(resp.Body)
}

func (c *Client) readCache(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > cacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	return os.ReadFile(path)
}

func (c *Client) writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
