package cache

import (
	"regexp"

	"github.com/mk-5/fjira/internal/jira"
)

// childrenJqlRegexp recognizes the exact epic-children query fjira's issue
// view issues (see issues.loadEpicChildren), so its results land under
// children/<KEY> instead of an opaque search hash.
var childrenJqlRegexp = regexp.MustCompile(`^parent = "([^"]+)" ORDER BY key ASC$`)

// apiWithCacheWrite decorates jira.Api with cache write-through: every
// successful fetch also lands in the on-disk cache, so normal fjira use keeps
// it warm. Cache writes are best-effort — a write failure never fails the
// fetch, and no fetch is ever served from the cache (fjira stays live-only;
// the cache exists for external readers).
type apiWithCacheWrite struct {
	jira.Api
	store *Store
}

// WrapApi returns api decorated with write-through into store.
func WrapApi(api jira.Api, store *Store) jira.Api {
	return &apiWithCacheWrite{Api: api, store: store}
}

func (c *apiWithCacheWrite) GetIssueDetailed(id string) (*jira.Issue, error) {
	issue, err := c.Api.GetIssueDetailed(id)
	if err == nil && issue != nil {
		_ = c.store.WriteIssue(IssueEntryFromJira(issue, c.Api.GetApiUrl()))
	}
	return issue, err
}

func (c *apiWithCacheWrite) SearchJql(jql string) ([]jira.Issue, error) {
	issues, err := c.Api.SearchJql(jql)
	if err == nil {
		c.cacheSearch(jql, issues)
	}
	return issues, err
}

// SearchJqlPageable caches the first page only: replace-in-place means a
// later page would otherwise overwrite the entry with a mid-list slice.
// Page 0 is a truthful "most relevant window" snapshot for the summary tier.
func (c *apiWithCacheWrite) SearchJqlPageable(jql string, page int32, pageSize int32) ([]jira.Issue, int32, int32, error) {
	issues, total, size, err := c.Api.SearchJqlPageable(jql, page, pageSize)
	if err == nil && page == 0 {
		c.cacheSearch(jql, issues)
	}
	return issues, total, size, err
}

func (c *apiWithCacheWrite) cacheSearch(jql string, issues []jira.Issue) {
	if m := childrenJqlRegexp.FindStringSubmatch(jql); m != nil {
		_ = c.store.WriteChildren(ChildrenEntry{Parent: m[1], Issues: SummariesFromJira(issues)})
		return
	}
	_ = c.store.WriteSearch(SearchEntry{Jql: jql, Issues: SummariesFromJira(issues)})
}
