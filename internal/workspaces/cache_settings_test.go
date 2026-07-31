package workspaces

import (
	"os"
	"testing"

	os2 "github.com/mk-5/fjira/internal/os"
	"github.com/stretchr/testify/assert"
)

func Test_ReadCacheSettings_defaults(t *testing.T) {
	// given
	tempDir := t.TempDir()
	_ = os2.SetUserHomeDir(tempDir)
	t.Setenv(CacheFilesEnv, "")
	t.Setenv(CacheExpiresEnv, "")

	// when
	cfg := ReadCacheSettings()

	// then
	assert.False(t, cfg.Files)
	assert.Equal(t, DefaultCacheExpires, cfg.Expires)
}

func Test_ReadCacheSettings_fromYaml(t *testing.T) {
	// given
	tempDir := t.TempDir()
	_ = os2.SetUserHomeDir(tempDir)
	t.Setenv(CacheFilesEnv, "")
	t.Setenv(CacheExpiresEnv, "")
	s := &userHomeSettingsStorage{}
	path, err := s.settingsFilePath()
	assert.NoError(t, err)
	yml := "current: default\nmanage:\n  cache:\n    files: true\n    expires: 10d\n"
	assert.NoError(t, os.WriteFile(path, []byte(yml), 0o644))

	// when
	cfg := ReadCacheSettings()

	// then
	assert.True(t, cfg.Files)
	assert.Equal(t, "10d", cfg.Expires)
}

func Test_ReadCacheSettings_envOverrides(t *testing.T) {
	// given
	tempDir := t.TempDir()
	_ = os2.SetUserHomeDir(tempDir)
	t.Setenv(CacheFilesEnv, "true")
	t.Setenv(CacheExpiresEnv, "36h")

	// when
	cfg := ReadCacheSettings()

	// then
	assert.True(t, cfg.Files)
	assert.Equal(t, "36h", cfg.Expires)
}

func Test_settings_manageBlockSurvivesRewrite(t *testing.T) {
	// given - a hand-added manage: block in fjira.yaml
	tempDir := t.TempDir()
	_ = os2.SetUserHomeDir(tempDir)
	s := &userHomeSettingsStorage{}
	path, err := s.settingsFilePath()
	assert.NoError(t, err)
	yml := "current: default\nmanage:\n  cache:\n    files: true\n"
	assert.NoError(t, os.WriteFile(path, []byte(yml), 0o644))

	// when - fjira rewrites the whole file for an unrelated change
	assert.NoError(t, s.WriteIssueFilters("default/TEST", ProjectIssueFilters{Label: "x"}))

	// then
	settings, err := s.createOrGetSettings()
	assert.NoError(t, err)
	assert.True(t, settings.Manage.Cache.Files)
}
