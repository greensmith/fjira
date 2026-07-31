package cache

import (
	"testing"

	"github.com/mk-5/fjira/internal/jira"
	"github.com/stretchr/testify/assert"
)

func Test_IssueEntryFromJira(t *testing.T) {
	// given
	issue := testIssue()

	// when
	entry := IssueEntryFromJira(issue, "https://jira.example.com/")

	// then
	assert.Equal(t, "TEST-501", entry.Key)
	assert.Equal(t, "https://jira.example.com/browse/TEST-501", entry.Url)
	assert.Equal(t, "Story", entry.Type)
	assert.Equal(t, StatusEntry{Name: "In Progress", Category: "indeterminate"}, entry.Status)
	assert.Equal(t, "A User", entry.Assignee)
	assert.Equal(t, "B User", entry.Reporter)
	assert.Equal(t, "TEST-490", entry.Parent)
	assert.Len(t, entry.Comments, 1)
	assert.Equal(t, "C User", entry.Comments[0].Author)
	assert.Equal(t, "human", entry.Comments[0].AuthorType)
	assert.Equal(t, "hi", entry.Comments[0].Body)
}

func Test_authorType(t *testing.T) {
	tests := []struct {
		accountType string
		want        string
	}{
		{"atlassian", "human"},
		{"customer", "human"},
		{"app", "bot"},
		{"", ""},      // jira server: accountType absent, not derivable
		{"weird", ""}, // unknown future kinds stay unclassified
	}
	for _, tt := range tests {
		u := jira.User{AccountType: tt.accountType}
		assert.Equal(t, tt.want, authorType(u), "accountType=%q", tt.accountType)
	}
}

func Test_SummariesFromJira(t *testing.T) {
	// given
	issue := jira.Issue{Key: "TEST-1"}
	issue.Fields.Summary = "child"
	issue.Fields.Status.Name = "Done"
	issue.Fields.Status.StatusCategory.Key = "done"

	// when
	summaries := SummariesFromJira([]jira.Issue{issue})

	// then
	assert.Equal(t, []IssueSummaryEntry{{
		Key:     "TEST-1",
		Summary: "child",
		Status:  StatusEntry{Name: "Done", Category: "done"},
	}}, summaries)
}
