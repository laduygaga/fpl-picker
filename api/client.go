package api

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	baseURL  = "https://fantasy.premierleague.com/api"
	cacheDir = ".fpl-cache"

	// Split TTLs for bootstrap-static:
	//   - teams/events/element_types change rarely (kickoff calendar, squad rules)
	//   - elements (player prices, ownership, status) change weekly
	// fixtures change as the schedule updates.
	//
	// FPL returns bootstrap-static as ONE response containing all four
	// groups, so we cannot refetch selectively. Pragmatic policy: if ANY
	// field is stale, refetch the whole response and reset ALL fetched_at
	// timestamps. The per-field structure exists so freshness is explicit
	// and future partial-refresh logic can slot in without breaking the
	// contract.
	ttlStatic  = 24 * time.Hour
	ttlDynamic = 1 * time.Hour
)

// Client fetches and caches FPL API data.
type Client struct {
	http     *http.Client
	cacheDir string
	ctx      context.Context
}

// cacheMeta tracks per-field fetch timestamps for split-TTL caching.
// Bootstrap-static writes four fields; fixtures writes one.
type cacheMeta struct {
	Teams        time.Time `json:"teams,omitzero"`
	Events       time.Time `json:"events,omitzero"`
	ElementTypes time.Time `json:"element_types,omitzero"`
	Elements     time.Time `json:"elements,omitzero"`
	Fixtures     time.Time `json:"fixtures,omitzero"`
}

// fieldSpec declares one tracked field within a cached endpoint and its TTL.
type fieldSpec struct {
	meta string        // cacheMeta field name
	ttl  time.Duration // freshness window
}

// bootstrapFields returns the per-field TTLs for the bootstrap-static endpoint.
func bootstrapFields() []fieldSpec {
	return []fieldSpec{
		{"Teams", ttlStatic},
		{"Events", ttlStatic},
		{"ElementTypes", ttlStatic},
		{"Elements", ttlDynamic},
	}
}

// fixturesFields returns the per-field TTLs for the fixtures endpoint.
func fixturesFields() []fieldSpec {
	return []fieldSpec{
		{"Fixtures", ttlDynamic},
	}
}

// NewClient creates an FPL API client with sensible defaults.
func NewClient(ctx context.Context) *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		cacheDir: cacheDir,
		ctx:      ctx,
	}
}

// FetchBootstrap fetches the master data: all players, teams, gameweeks.
func (c *Client) FetchBootstrap() (*BootstrapResponse, error) {
	var resp BootstrapResponse
	if err := c.fetchCached("bootstrap-static", bootstrapFields(), &resp); err != nil {
		return nil, fmt.Errorf("fetching bootstrap data: %w", err)
	}
	return &resp, nil
}

// FetchFixtures fetches all fixtures for the season.
func (c *Client) FetchFixtures() ([]Fixture, error) {
	var fixtures []Fixture
	if err := c.fetchCached("fixtures", fixturesFields(), &fixtures); err != nil {
		return nil, fmt.Errorf("fetching fixtures: %w", err)
	}
	return fixtures, nil
}

// ClearCache removes all cached data, forcing fresh fetches.
func (c *Client) ClearCache() error {
	return os.RemoveAll(c.cacheDir)
}

// fetchCached serves the endpoint from cache when every tracked field is
// within its TTL; otherwise it refetches and rewrites body + etag + meta.
// On a successful HTTP response (200 or 304), ALL tracked fields' timestamps
// are reset because FPL returns the whole response in one shot.
func (c *Client) fetchCached(endpoint string, fields []fieldSpec, target any) error {
	cacheFile := filepath.Join(c.cacheDir, endpoint+".json")
	metaFile := filepath.Join(c.cacheDir, endpoint+".meta.json")
	etagFile := filepath.Join(c.cacheDir, endpoint+".etag")

	if data, err := c.readCache(cacheFile, metaFile, etagFile, fields); err == nil {
		if jsonErr := json.Unmarshal(data, target); jsonErr == nil {
			return nil
		}
		// Corrupt cache: fall through to refetch.
	}

	etag, _ := os.ReadFile(etagFile)
	data, newETag, err := c.fetchRaw(endpoint, strings.TrimSpace(string(etag)))
	if err != nil {
		return err
	}

	// Best-effort cache writes — never fail the request if caching fails.
	_ = c.writeCache(cacheFile, data)
	if newETag != "" {
		_ = os.WriteFile(etagFile, []byte(newETag), 0o644)
	}
	_ = c.writeMeta(metaFile, fields, time.Now())

	return json.Unmarshal(data, target)
}

// fetchRaw performs the HTTP GET with gzip request + ETag conditional + 304
// fallback to cached body. Returns the response body bytes and (if present)
// the new ETag value to persist.
func (c *Client) fetchRaw(endpoint, etag string) ([]byte, string, error) {
	url := fmt.Sprintf("%s/%s/", baseURL, endpoint)

	req, err := http.NewRequestWithContext(c.ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "fpl-picker/1.0")
	req.Header.Set("Accept-Encoding", "gzip")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("request to %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	// 304 Not Modified: server confirms cached body is current; nothing to download.
	if resp.StatusCode == http.StatusNotModified {
		cacheFile := filepath.Join(c.cacheDir, endpoint+".json")
		data, readErr := os.ReadFile(cacheFile)
		if readErr != nil {
			return nil, "", fmt.Errorf("304 but cache missing for %s: %w", endpoint, readErr)
		}
		return data, etag, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("FPL API returned %d for %s", resp.StatusCode, endpoint)
	}

	body, err := readMaybeGzip(resp)
	if err != nil {
		return nil, "", err
	}

	return body, resp.Header.Get("ETag"), nil
}

// readMaybeGzip transparently decompresses the response body when the server
// advertised gzip encoding. For plain JSON it returns the raw bytes.
func readMaybeGzip(resp *http.Response) ([]byte, error) {
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		return io.ReadAll(gz)
	}
	return io.ReadAll(resp.Body)
}

// readCache returns the cached body when every tracked field is within its TTL.
// When metaFile is missing, the cache file's ModTime is used as the fallback
// fetch timestamp for every field — this preserves backward compatibility with
// cache files written before the split-TTL upgrade.
func (c *Client) readCache(cacheFile, metaFile, etagFile string, fields []fieldSpec) ([]byte, error) {
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, err
	}

	meta, metaErr := c.loadMeta(metaFile, cacheFile, fields)
	if metaErr != nil {
		return nil, metaErr
	}

	now := time.Now()
	for _, f := range fields {
		fetched := metaField(meta, f.meta)
		if fetched.IsZero() {
			return nil, fmt.Errorf("cache field %s has no timestamp", f.meta)
		}
		if now.Sub(fetched) > f.ttl {
			return nil, fmt.Errorf("cache stale for field %s", f.meta)
		}
	}
	_ = etagFile // read lazily by fetchRaw; not needed for freshness check
	return data, nil
}

// loadMeta reads the per-field timestamps. If the meta sidecar is missing,
// it falls back to the cache file's ModTime for every tracked field so
// pre-existing caches keep working with field-appropriate TTLs.
func (c *Client) loadMeta(metaFile, cacheFile string, fields []fieldSpec) (cacheMeta, error) {
	var meta cacheMeta
	data, err := os.ReadFile(metaFile)
	if err == nil {
		if jsonErr := json.Unmarshal(data, &meta); jsonErr == nil {
			return meta, nil
		}
		// Corrupt meta: treat as missing, fall through.
	}

	info, statErr := os.Stat(cacheFile)
	if statErr != nil {
		return meta, statErr
	}
	for _, f := range fields {
		setMetaField(&meta, f.meta, info.ModTime())
	}
	return meta, nil
}

// writeMeta stamps every tracked field of metaFile with t (i.e. "now").
// One write per fetch is fine — the file is tiny and rarely touched.
func (c *Client) writeMeta(metaFile string, fields []fieldSpec, t time.Time) error {
	meta := cacheMeta{}
	for _, f := range fields {
		setMetaField(&meta, f.meta, t)
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(metaFile), 0o755); err != nil {
		return err
	}
	return os.WriteFile(metaFile, data, 0o644)
}

func (c *Client) writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// metaField / setMetaField use a tiny switch instead of reflection — the
// cacheMeta struct has exactly five fields, all of the same type, and the
// indirection keeps the call sites readable without a runtime cost.
func metaField(m cacheMeta, name string) time.Time {
	switch name {
	case "Teams":
		return m.Teams
	case "Events":
		return m.Events
	case "ElementTypes":
		return m.ElementTypes
	case "Elements":
		return m.Elements
	case "Fixtures":
		return m.Fixtures
	}
	return time.Time{}
}

func setMetaField(m *cacheMeta, name string, t time.Time) {
	switch name {
	case "Teams":
		m.Teams = t
	case "Events":
		m.Events = t
	case "ElementTypes":
		m.ElementTypes = t
	case "Elements":
		m.Elements = t
	case "Fixtures":
		m.Fixtures = t
	}
}
