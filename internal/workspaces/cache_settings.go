package workspaces

import (
	"os"
	"strings"
)

const (
	CacheFilesEnv       = "FJIRA_CACHE_FILES"
	CacheExpiresEnv     = "FJIRA_CACHE_EXPIRES"
	DefaultCacheExpires = "3w"
)

// ReadCacheSettings resolves manage.cache from fjira.yaml, applies the
// FJIRA_CACHE_FILES / FJIRA_CACHE_EXPIRES env overrides, and fills defaults
// (files: false, expires: 3w). Any read error degrades to the defaults —
// cache configuration must never block startup.
func ReadCacheSettings() CacheSettings {
	cfg := CacheSettings{}
	s := NewUserHomeSettingsStorage().(*userHomeSettingsStorage)
	if settings, err := s.createOrGetSettings(); err == nil {
		cfg = settings.Manage.Cache
	}
	if v := os.Getenv(CacheFilesEnv); v != "" {
		cfg.Files = parseBoolEnv(v)
	}
	if v := os.Getenv(CacheExpiresEnv); v != "" {
		cfg.Expires = v
	}
	if cfg.Expires == "" {
		cfg.Expires = DefaultCacheExpires
	}
	return cfg
}

func parseBoolEnv(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
