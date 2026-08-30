package cache

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var cacheDir string

func init() {
	home, err := os.UserHomeDir()
	if err == nil {
		cacheDir = filepath.Join(home, ".cache", "verifisci")
		_ = os.MkdirAll(cacheDir, 0755)
	}
}

func Key(prefix string, args ...string) string {
	raw := prefix + strings.Join(args, "|")
	hash := md5.Sum([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func Get[T any](key string, maxAge time.Duration) (*T, bool) {
	if cacheDir == "" {
		return nil, false
	}
	path := filepath.Join(cacheDir, key)
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if time.Since(info.ModTime()) > maxAge {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var res T
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, false
	}
	return &res, true
}

func Set(key string, data any) error {
	if cacheDir == "" {
		return nil
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	path := filepath.Join(cacheDir, key)
	return os.WriteFile(path, bytes, 0644)
}
