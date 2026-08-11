package cache

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/stretchr/testify/assert"
)

// fakeApi implements only the methods the decorator exercises; everything
// else nil-panics via the embedded interface, which is fine for these tests.
type fakeApi struct {
	jira.Api
	issue     *jira.Issue
	issues    []jira.Issue
	searchErr error
}

func (f *fakeApi) GetApiUrl() string { return "https://jira.example.com/" }

func (f *fakeApi) GetIssueDetailed(id string) (*jira.Issue, error) {
	if f.issue == nil {
		return nil, errors.New("boom")
	}
	return f.issue, nil
}

func (f *fakeApi) SearchJql(jql string) ([]jira.Issue, error) {
	return f.issues, f.searchErr
}

func (f *fakeApi) SearchJqlPageable(jql string, page int32, pageSize int32) ([]jira.Issue, int32, int32, error) {
	return f.issues, int32(len(f.issues)), pageSize, f.searchErr
}

func testIssue() *jira.Issue {
	issue := &jira.Issue{Key: "TEST-501"}
	issue.Fields.Summary = "Do the thing"
	issue.Fields.Type.Name = "Story"
	issue.Fields.Status = jira.Status{Name: "In Progress"}
	issue.Fields.Status.StatusCategory.Key = "indeterminate"
	issue.Fields.Assignee.DisplayName = "A User"
	issue.Fields.Reporter.DisplayName = "B User"
	issue.Fields.Parent.Key = "TEST-490"
	issue.Fields.Updated = "2026-07-29T09:11:00.000-0700"
	comment := jira.Comment{Body: "hi", Created: "2026-07-28T16:40:00.000-0700"}
	comment.Author.DisplayName = "C User"
	comment.Author.AccountType = "atlassian"
	issue.Fields.Comment.Comments = []jira.Comment{comment}
	return issue
}

func wrapped(t *testing.T, fake *fakeApi) (jira.Api, *Store) {
	t.Helper()
	store := NewStore(t.TempDir(), "default")
	store.now = func() time.Time { return time.Date(2026, 7, 30, 8, 0, 0, 0, time.Local) }
	return WrapApi(fake, store), store
}

func Test_apiWithCacheWrite_GetIssueDetailed(t *testing.T) {
	// given
	api, store := wrapped(t, &fakeApi{issue: testIssue()})

	// when
	issue, err := api.GetIssueDetailed("TEST-501")

	// then - fetch passes through and the detail tier lands on disk
	assert.NoError(t, err)
	assert.Equal(t, "TEST-501", issue.Key)
	files, _ := filepath.Glob(filepath.Join(store.root, "default", "issue", "TEST-501.*.yaml"))
	assert.Len(t, files, 1)
}

func Test_apiWithCacheWrite_fetchErrorWritesNothing(t *testing.T) {
	// given
	api, store := wrapped(t, &fakeApi{issue: nil, searchErr: errors.New("boom")})

	// when
	_, issueErr := api.GetIssueDetailed("TEST-501")
	_, searchErr := api.SearchJql("project = TEST")

	// then
	assert.Error(t, issueErr)
	assert.Error(t, searchErr)
	files, _ := filepath.Glob(filepath.Join(store.root, "default", "*", "*"))
	assert.Empty(t, files)
}

func Test_apiWithCacheWrite_SearchJql(t *testing.T) {
	// given
	api, store := wrapped(t, &fakeApi{issues: []jira.Issue{{Key: "TEST-1"}}})

	// when
	_, err := api.SearchJql("project = TEST")

	// then
	assert.NoError(t, err)
	path := filepath.Join(store.root, "default", "search", JqlHash("project = TEST")+".2026-07-30.yaml")
	assert.FileExists(t, path)
}

func Test_apiWithCacheWrite_childrenJqlLandsUnderChildren(t *testing.T) {
	// given - the exact epic-children query the issue view issues
	api, store := wrapped(t, &fakeApi{issues: []jira.Issue{{Key: "TEST-101"}}})

	// when
	_, err := api.SearchJql(`parent = "TEST-100" ORDER BY key ASC`)

	// then - children/<KEY>, and no search-hash duplicate
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(store.root, "default", "children", "TEST-100.2026-07-30.yaml"))
	searchFiles, _ := filepath.Glob(filepath.Join(store.root, "default", "search", "*"))
	assert.Empty(t, searchFiles)
}

func Test_apiWithCacheWrite_pageableCachesFirstPageOnly(t *testing.T) {
	// given
	api, store := wrapped(t, &fakeApi{issues: []jira.Issue{{Key: "TEST-1"}}})

	// when
	_, _, _, err := api.SearchJqlPageable("project = TEST", 1, 100)

	// then - page 1 must not clobber the page-0 snapshot
	assert.NoError(t, err)
	files, _ := filepath.Glob(filepath.Join(store.root, "default", "search", "*"))
	assert.Empty(t, files)

	// when
	_, _, _, err = api.SearchJqlPageable("project = TEST", 0, 100)

	// then
	assert.NoError(t, err)
	files, _ = filepath.Glob(filepath.Join(store.root, "default", "search", "*"))
	assert.Len(t, files, 1)
}
