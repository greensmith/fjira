package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	os2 "github.com/mk-5/fjira/internal/os"
	"github.com/mk-5/fjira/internal/workspaces"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func headlessTestServer(t *testing.T) (*httptest.Server, *string) {
	t.Helper()
	var lastJql string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		switch r.URL.Path {
		case "/rest/api/2/issue/TEST-1":
			_, _ = w.Write([]byte(`{"key": "TEST-1", "fields": {"summary": "a summary", "status": {"name": "Done", "statusCategory": {"key": "done"}}}}`))
		default:
			lastJql = r.URL.Query().Get("jql")
			_, _ = w.Write([]byte(`{"total": 1, "issues": [{"key": "TEST-2", "fields": {"summary": "child"}}]}`))
		}
	}))
	t.Cleanup(server.Close)
	return server, &lastJql
}

// runHeadlessCmd executes cmd hermetically: temp home + cache dir, caching
// off unless cacheEnv is set, workspace settings pointing at the test server.
func runHeadlessCmd(t *testing.T, cmd *cobra.Command, serverUrl string, cacheEnv string, args ...string) (string, string, error) {
	t.Helper()
	_ = os2.SetUserHomeDir(t.TempDir())
	cacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cacheDir)
	t.Setenv(workspaces.CacheFilesEnv, cacheEnv)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	settings := &workspaces.WorkspaceSettings{JiraRestUrl: serverUrl, JiraUsername: "u", JiraToken: "t", Workspace: "testws"}
	err := cmd.ExecuteContext(context.WithValue(context.TODO(), CtxWorkspaceSettings, settings))
	return out.String(), cacheDir, err
}

func TestGetHeadlessIssueCmd(t *testing.T) {
	// given
	server, _ := headlessTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessIssueCmd(), server.URL, "", "TEST-1", "--json")

	// then - cache-entry-shaped JSON with an inlined envelope
	assert.NoError(t, err)
	var doc map[string]any
	assert.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Equal(t, float64(1), doc["v"])
	assert.Equal(t, "issue", doc["kind"])
	assert.Equal(t, "testws", doc["workspace"])
	assert.Equal(t, "TEST-1", doc["key"])
	assert.Equal(t, "a summary", doc["summary"])
	assert.Equal(t, map[string]any{"name": "Done", "category": "done"}, doc["status"])
	assert.Contains(t, doc["url"], "/browse/TEST-1")
}

func TestGetHeadlessIssueCmd_requiresJsonFlag(t *testing.T) {
	// given
	server, _ := headlessTestServer(t)

	// when
	_, _, err := runHeadlessCmd(t, GetHeadlessIssueCmd(), server.URL, "", "TEST-1")

	// then
	assert.ErrorContains(t, err, "json")
}

func TestGetHeadlessIssueCmd_warmsCacheWhenEnabled(t *testing.T) {
	// given
	server, _ := headlessTestServer(t)

	// when
	_, cacheDir, err := runHeadlessCmd(t, GetHeadlessIssueCmd(), server.URL, "1", "TEST-1", "--json")

	// then - the headless fetch went through the write-through decorator
	assert.NoError(t, err)
	files, _ := filepath.Glob(filepath.Join(cacheDir, "fjira", "testws", "issue", "TEST-1.*.yaml"))
	assert.Len(t, files, 1)
}

func TestGetHeadlessSearchCmd(t *testing.T) {
	// given
	server, lastJql := headlessTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessSearchCmd(), server.URL, "", "project = TEST", "--json")

	// then
	assert.NoError(t, err)
	assert.Equal(t, "project = TEST", *lastJql)
	var doc map[string]any
	assert.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Equal(t, "search", doc["kind"])
	assert.Equal(t, "project = TEST", doc["jql"])
	issues := doc["issues"].([]any)
	assert.Len(t, issues, 1)
	assert.Equal(t, "TEST-2", issues[0].(map[string]any)["key"])
}

func TestGetHeadlessChildrenCmd(t *testing.T) {
	// given
	server, lastJql := headlessTestServer(t)

	// when
	out, _, err := runHeadlessCmd(t, GetHeadlessChildrenCmd(), server.URL, "", "TEST-100", "--json")

	// then - the canonical children JQL was sent, parent echoed in the doc
	assert.NoError(t, err)
	assert.Equal(t, `parent = "TEST-100" ORDER BY key ASC`, *lastJql)
	var doc map[string]any
	assert.NoError(t, json.Unmarshal([]byte(out), &doc))
	assert.Equal(t, "children", doc["kind"])
	assert.Equal(t, "TEST-100", doc["parent"])
	assert.Len(t, doc["issues"].([]any), 1)
}

func TestGetHeadlessChildrenCmd_cachesUnderChildren(t *testing.T) {
	// given
	server, _ := headlessTestServer(t)

	// when
	_, cacheDir, err := runHeadlessCmd(t, GetHeadlessChildrenCmd(), server.URL, "1", "TEST-100", "--json")

	// then - canonical JQL means the decorator files it under children/
	assert.NoError(t, err)
	files, _ := filepath.Glob(filepath.Join(cacheDir, "fjira", "testws", "children", "TEST-100.*.yaml"))
	assert.Len(t, files, 1)
}
