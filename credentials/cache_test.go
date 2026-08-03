package credentials

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// useTempCachePath redirects CachePath to a temp file inside t.TempDir().
// Returns the path so tests can read it back if needed.
func useTempCachePath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cache.bin")
	original := CachePath
	CachePath = path
	t.Cleanup(func() { CachePath = original })
	return path
}

func TestSaveLoadRoundtrip(t *testing.T) {
	useTempCachePath(t)

	email := "user@example.com"
	password := "hunter2"
	passphrase := "correct horse battery staple"

	if err := Save(email, password, passphrase); err != nil {
		t.Fatalf("Save: %v", err)
	}

	gotEmail, gotPassword, err := Load(passphrase)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotEmail != email {
		t.Errorf("Email = %q, want %q", gotEmail, email)
	}
	if gotPassword != password {
		t.Errorf("Password = %q, want %q", gotPassword, password)
	}
}

func TestLoadWrongPassphrase(t *testing.T) {
	useTempCachePath(t)

	if err := Save("u@e.com", "pw", "right-pass"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	_, _, err := Load("wrong-pass")
	if !errors.Is(err, ErrWrongPassphrase) {
		t.Fatalf("Load wrong passphrase: err = %v, want ErrWrongPassphrase", err)
	}
	if !IsWrongPassphrase(err) {
		t.Error("IsWrongPassphrase should return true for the same error")
	}
}

func TestLoadMissingFile(t *testing.T) {
	useTempCachePath(t)

	_, _, err := Load("any")
	if !errors.Is(err, ErrCacheMissing) {
		t.Fatalf("Load missing: err = %v, want ErrCacheMissing", err)
	}
}

func TestExistsAndClear(t *testing.T) {
	path := useTempCachePath(t)

	if Exists() {
		t.Fatal("Exists should be false on empty cache")
	}
	if err := Save("a@b", "c", "pp"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if !Exists() {
		t.Fatal("Exists should be true after Save")
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if Exists() {
		t.Fatal("Exists should be false after Clear")
	}
	// Clearing twice is a no-op, not an error.
	if err := Clear(); err != nil {
		t.Errorf("Clear (idempotent): %v", err)
	}
	_ = path
}

func TestSaveFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions not enforced on Windows")
	}
	path := useTempCachePath(t)

	if err := Save("a@b", "c", "pp"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestSaveOverwritesExisting(t *testing.T) {
	useTempCachePath(t)

	if err := Save("a@b", "first", "pp"); err != nil {
		t.Fatalf("Save1: %v", err)
	}
	if err := Save("c@d", "second", "pp"); err != nil {
		t.Fatalf("Save2: %v", err)
	}
	gotEmail, gotPassword, err := Load("pp")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if gotEmail != "c@d" || gotPassword != "second" {
		t.Errorf("got %s/%s, want c@d/second", gotEmail, gotPassword)
	}
}

func TestSaveRejectsEmptyPassphrase(t *testing.T) {
	useTempCachePath(t)
	if err := Save("a@b", "c", ""); err == nil {
		t.Error("Save with empty passphrase should fail")
	}
}

func TestCorruptFileFails(t *testing.T) {
	path := useTempCachePath(t)
	if err := os.WriteFile(path, []byte("not a json envelope"), 0o600); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}
	_, _, err := Load("anything")
	if err == nil || errors.Is(err, ErrWrongPassphrase) {
		t.Errorf("corrupt file should return parse error, got %v", err)
	}
}

func TestVersionMismatch(t *testing.T) {
	path := useTempCachePath(t)
	bad := `{"v":99,"kdf":"pbkdf2-sha256","iter":100000,"salt":"AAAA","nonce":"BBBB","ct":"CCCC"}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := Load("pp")
	if err == nil || !strings.Contains(err.Error(), "unsupported cache version") {
		t.Errorf("got %v, want unsupported cache version error", err)
	}
}

func TestUnknownKDF(t *testing.T) {
	path := useTempCachePath(t)
	bad := `{"v":1,"kdf":"scrypt","iter":100000,"salt":"AAAA","nonce":"BBBB","ct":"CCCC"}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := Load("pp")
	if err == nil || !strings.Contains(err.Error(), "unsupported KDF") {
		t.Errorf("got %v, want unsupported KDF error", err)
	}
}
