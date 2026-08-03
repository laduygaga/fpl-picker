// Package credentials implements an encrypted on-disk cache for the user's
// FPL login email and password. The cache file is AES-256-GCM ciphertext; the
// symmetric key is derived from a user-supplied passphrase via PBKDF2-HMAC-SHA256
// (100,000 iterations, 16-byte random salt). The 12-byte nonce is random per
// write. The cleartext password never touches disk.
//
// File format (JSON, single line):
//
//	{"v":1,"kdf":"pbkdf2-sha256","iter":100000,"salt":"<b64>","nonce":"<b64>","ct":"<b64>"}
package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
)

const (
	// CacheFile is the file (relative to CacheDir) that stores the encrypted blob.
	CacheFile = ".cached"

	// CacheDirName is the directory (relative to user config) holding CacheFile.
	CacheDirName = ".config/fpl-picker"

	// keyLen is the AES-256 key length in bytes.
	keyLen = 32

	// saltLen is the PBKDF2 salt length in bytes.
	saltLen = 16

	// nonceLen is the AES-GCM nonce length in bytes.
	nonceLen = 12

	// pbkdf2Iter is the PBKDF2 iteration count.
	pbkdf2Iter = 100_000

	// formatVersion is the on-disk cache format version.
	formatVersion = 1
)

var (
	// ErrWrongPassphrase is returned when decryption fails (GCM authentication tag mismatch).
	ErrWrongPassphrase = errors.New("credentials: wrong passphrase or corrupt cache")
	// ErrCacheMissing is returned when Load is called but the cache file does not exist.
	ErrCacheMissing = errors.New("credentials: cache file not found")
)

type envelope struct {
	V     int    `json:"v"`
	KDF   string `json:"kdf"`
	Iter  int    `json:"iter"`
	Salt  string `json:"salt"`
	Nonce string `json:"nonce"`
	CT    string `json:"ct"`
}

// defaultCachePath returns the production cache path: ~/.config/fpl-picker/.cached
func defaultCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home directory: %w", err)
	}
	return filepath.Join(home, CacheDirName, CacheFile), nil
}

// CachePath returns the current cache file path. Overridable for tests.
var CachePath = func() string {
	p, err := defaultCachePath()
	if err != nil {
		// Fall back to a relative path; Save/Load will surface the error.
		return filepath.Join(CacheDirName, CacheFile)
	}
	return p
}()

// sha256Func is a hash.Hash factory wrapped in a non-generic signature so the
// compiler can infer the type parameter of pbkdf2.Key (generic in Go 1.24+).
func sha256Func() hash.Hash { return sha256.New() }

func deriveKey(passphrase string, salt []byte, iter int) ([]byte, error) {
	return pbkdf2.Key(sha256Func, passphrase, salt, iter, keyLen)
}

// encrypt seals the cleartext under the given passphrase and returns a JSON envelope.
func encrypt(plain, passphrase string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	key, err := deriveKey(passphrase, salt, pbkdf2Iter)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)

	env := envelope{
		V:     formatVersion,
		KDF:   "pbkdf2-sha256",
		Iter:  pbkdf2Iter,
		Salt:  base64.StdEncoding.EncodeToString(salt),
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		CT:    base64.StdEncoding.EncodeToString(ct),
	}
	return json.Marshal(env)
}

func newGCM(passphrase string, salt []byte, iter int) (cipher.AEAD, error) {
	key, err := deriveKey(passphrase, salt, iter)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("aes key: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	return gcm, nil
}

// decrypt opens the envelope and verifies the GCM tag with the given passphrase.
func decrypt(envBytes, passphrase string) (string, error) {
	var env envelope
	if err := json.Unmarshal([]byte(envBytes), &env); err != nil {
		return "", fmt.Errorf("parse envelope: %w", err)
	}
	if env.V != formatVersion {
		return "", fmt.Errorf("unsupported cache version: %d", env.V)
	}
	if env.KDF != "pbkdf2-sha256" {
		return "", fmt.Errorf("unsupported KDF: %s", env.KDF)
	}

	salt, err := base64.StdEncoding.DecodeString(env.Salt)
	if err != nil {
		return "", fmt.Errorf("decode salt: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return "", fmt.Errorf("decode nonce: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(env.CT)
	if err != nil {
		return "", fmt.Errorf("decode ct: %w", err)
	}

	gcm, err := newGCM(passphrase, salt, env.Iter)
	if err != nil {
		return "", err
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", ErrWrongPassphrase
	}
	return string(plain), nil
}

// Save encrypts and writes the credentials to CachePath. The file is created
// with 0600 permissions; the parent directory with 0700. An existing file is
// replaced atomically by writing to a temp file and renaming.
func Save(email, password, passphrase string) error {
	if passphrase == "" {
		return errors.New("credentials: empty passphrase")
	}
	plain := email + "\n" + password
	envBytes, err := encrypt(plain, passphrase)
	if err != nil {
		return err
	}

	path := CachePath
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".cached-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	// Ensure the temp file is removed if anything below fails.
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if _, err := tmp.Write(envBytes); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if _, err := tmp.Write([]byte("\n")); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write newline: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}
	// Belt-and-suspenders: enforce 0600 on the final path in case the rename
	// preserved looser permissions from a pre-existing file.
	_ = os.Chmod(path, 0o600)
	return nil
}

// Load reads and decrypts the cache file at CachePath. Returns ErrWrongPassphrase
// when decryption fails (GCM tag mismatch), or ErrCacheMissing when the file
// does not exist.
func Load(passphrase string) (string, string, error) {
	path := CachePath
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", ErrCacheMissing
		}
		return "", "", fmt.Errorf("read cache: %w", err)
	}

	plain, err := decrypt(string(data), passphrase)
	if err != nil {
		return "", "", err
	}

	for i := 0; i < len(plain); i++ {
		if plain[i] == '\n' {
			return plain[:i], plain[i+1:], nil
		}
	}
	return "", "", errors.New("credentials: malformed cache plaintext")
}

// Exists returns true when the cache file is present on disk.
func Exists() bool {
	_, err := os.Stat(CachePath)
	return err == nil
}

// Clear removes the cache file. It is not an error to call Clear when the
// cache is already absent.
func Clear() error {
	err := os.Remove(CachePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// IsWrongPassphrase is a convenience predicate for callers that want to
// match on the specific error without importing the sentinel directly.
func IsWrongPassphrase(err error) bool {
	return errors.Is(err, ErrWrongPassphrase)
}
