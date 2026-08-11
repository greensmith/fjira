package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func testStore(t *testing.T, at time.Time) *Store {
	t.Helper()
	s := NewStore(t.TempDir(), "default")
	s.now = func() time.Time { return at }
	return s
}

func Test_store_WriteIssue(t *testing.T) {
	// given
	fetched := time.Date(2026, 7, 30, 14, 22, 5, 0, time.Local)
	s := testStore(t, fetched)

	// when
	err := s.WriteIssue(IssueEntry{
		Key:      "TEST-501",
		Summary:  "Disconnect surface map",
		Status:   StatusEntry{Name: "In Progress", Category: "indeterminate"},
		Comments: []CommentEntry{{Author: "A User", AuthorType: "human", Body: "hello"}},
	})

	// then - file named <key>.<local-date>.yaml with a stamped envelope
	assert.NoError(t, err)
	path := filepath.Join(s.root, "default", "issue", "TEST-501.2026-07-30.yaml")
	assert.FileExists(t, path)
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	var read IssueEntry
	assert.NoError(t, yaml.Unmarshal(data, &read))
	assert.Equal(t, SchemaVersion, read.V)
	assert.Equal(t, "issue", read.Kind)
	assert.Equal(t, "default", read.Workspace)
	assert.Equal(t, fetched.Format(time.RFC3339), read.FetchedAt)
	assert.Equal(t, "TEST-501", read.Key)
	assert.Equal(t, "indeterminate", read.Status.Category)
	assert.Equal(t, "hello", read.Comments[0].Body)
	// envelope fields are inlined at the top level, not nested
	var raw map[string]any
	assert.NoError(t, yaml.Unmarshal(data, &raw))
	assert.Equal(t, 1, raw["v"])
	assert.NotContains(t, raw, "envelope")
}

func Test_store_replaceInPlace(t *testing.T) {
	// given - an entry cached yesterday
	s := testStore(t, time.Date(2026, 7, 29, 8, 0, 0, 0, time.Local))
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-1"}))

	// when - the same entity is refreshed today
	s.now = func() time.Time { return time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local) }
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-1"}))

	// then - exactly one file remains, the new date; no temp leftovers
	files, err := filepath.Glob(filepath.Join(s.root, "default", "issue", "*"))
	assert.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "TEST-1.2026-07-30.yaml", filepath.Base(files[0]))
}

func Test_store_replaceInPlace_distinctIds(t *testing.T) {
	// given - two entities sharing a key prefix
	s := testStore(t, time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local))
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-1"}))

	// when
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-12"}))

	// then - TEST-1 must not be reaped by TEST-12's replace-in-place pass
	files, _ := filepath.Glob(filepath.Join(s.root, "default", "issue", "*"))
	assert.Len(t, files, 2)
}

func Test_store_WriteSearch_hashedFilename(t *testing.T) {
	// given
	s := testStore(t, time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local))
	jql := "project = TEST"

	// when
	err := s.WriteSearch(SearchEntry{Jql: jql, Issues: []IssueSummaryEntry{{Key: "TEST-1"}}})

	// then
	assert.NoError(t, err)
	path := filepath.Join(s.root, "default", "search", JqlHash(jql)+".2026-07-30.yaml")
	assert.FileExists(t, path)
	var read SearchEntry
	data, _ := os.ReadFile(path)
	assert.NoError(t, yaml.Unmarshal(data, &read))
	assert.Equal(t, "search", read.Kind)
	assert.Equal(t, jql, read.Jql)
	assert.Equal(t, "TEST-1", read.Issues[0].Key)
}

func Test_store_WriteChildren(t *testing.T) {
	// given
	s := testStore(t, time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local))

	// when
	err := s.WriteChildren(ChildrenEntry{Parent: "TEST-100", Issues: []IssueSummaryEntry{{Key: "TEST-101"}}})

	// then
	assert.NoError(t, err)
	path := filepath.Join(s.root, "default", "children", "TEST-100.2026-07-30.yaml")
	assert.FileExists(t, path)
	var read ChildrenEntry
	data, _ := os.ReadFile(path)
	assert.NoError(t, yaml.Unmarshal(data, &read))
	assert.Equal(t, "children", read.Kind)
	assert.Equal(t, "TEST-100", read.Parent)
}

func Test_store_emptyWorkspaceDefaults(t *testing.T) {
	s := NewStore(t.TempDir(), "")
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-1"}))
	files, _ := filepath.Glob(filepath.Join(s.root, "default", "issue", "*"))
	assert.Len(t, files, 1)
}

func Test_store_Reap(t *testing.T) {
	// given - one stale file, one fresh file, one stale temp leftover
	s := testStore(t, time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local))
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-1"}))
	assert.NoError(t, s.WriteIssue(IssueEntry{Key: "TEST-2"}))
	dir := filepath.Join(s.root, "default", "issue")
	stale := filepath.Join(dir, "TEST-1.2026-07-30.yaml")
	tmp := filepath.Join(dir, ".TEST-9.2026-06-01.yaml.tmp")
	assert.NoError(t, os.WriteFile(tmp, []byte("x"), 0o644))
	old := time.Date(2026, 6, 1, 0, 0, 0, 0, time.Local)
	assert.NoError(t, os.Chtimes(stale, old, old))
	assert.NoError(t, os.Chtimes(tmp, old, old))

	// when
	s.Reap(21 * 24 * time.Hour)

	// then
	assert.NoFileExists(t, stale)
	assert.NoFileExists(t, tmp)
	assert.FileExists(t, filepath.Join(dir, "TEST-2.2026-07-30.yaml"))
}
