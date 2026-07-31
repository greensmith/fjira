package fjira

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	os2 "github.com/mk-5/fjira/internal/os"
	"github.com/mk-5/fjira/internal/workspaces"
	"github.com/stretchr/testify/assert"
)

func Test_wrapApiWithCacheWrite_disabledByDefault(t *testing.T) {
	// given
	_ = os2.SetUserHomeDir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv(workspaces.CacheFilesEnv, "")
	api := jira.NewJiraApiMock(nil)

	// when
	wrapped := wrapApiWithCacheWrite(api, "default")

	// then - api untouched, cache dir never created
	assert.Equal(t, api, wrapped)
	assert.NoDirExists(t, filepath.Join(cacheDir, "fjira"))
}

func Test_wrapApiWithCacheWrite_writesThrough(t *testing.T) {
	// given
	_ = os2.SetUserHomeDir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv(workspaces.CacheFilesEnv, "1")
	api := jira.NewJiraApiMock(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"key": "TEST-1", "fields": {"summary": "cached"}}`))
	})

	// when
	wrapped := wrapApiWithCacheWrite(api, "myworkspace")
	_, err := wrapped.GetIssueDetailed("TEST-1")

	// then - the fetch landed in <cache>/fjira/<workspace>/issue/
	assert.NoError(t, err)
	files, globErr := filepath.Glob(filepath.Join(cacheDir, "fjira", "myworkspace", "issue", "TEST-1.*.yaml"))
	assert.NoError(t, globErr)
	assert.Len(t, files, 1)
}
