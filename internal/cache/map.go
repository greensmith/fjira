package cache

import (
	"fmt"
	"strings"

	"github.com/mk-5/fjira/internal/jira"
)

// IssueEntryFromJira maps a detailed jira issue onto the detail-tier cache
// entry. apiUrl is the jira base url (jiraRestUrl), used to build the human
// browse link.
func IssueEntryFromJira(issue *jira.Issue, apiUrl string) IssueEntry {
	comments := make([]CommentEntry, 0, len(issue.Fields.Comment.Comments))
	for _, c := range issue.Fields.Comment.Comments {
		comments = append(comments, CommentEntry{
			Author:     c.Author.DisplayName,
			AuthorType: authorType(c.Author),
			Created:    c.Created,
			Body:       c.Body,
		})
	}
	return IssueEntry{
		Key:      issue.Key,
		Summary:  issue.Fields.Summary,
		Url:      fmt.Sprintf("%s/browse/%s", strings.TrimSuffix(apiUrl, "/"), issue.Key),
		Type:     issue.Fields.Type.Name,
		Status:   statusEntry(issue.Fields.Status),
		Assignee: issue.Fields.Assignee.DisplayName,
		Reporter: issue.Fields.Reporter.DisplayName,
		Parent:   issue.Fields.Parent.Key,
		Updated:  issue.Fields.Updated,
		Comments: comments,
	}
}

// SummariesFromJira maps search results onto summary-tier entries.
func SummariesFromJira(issues []jira.Issue) []IssueSummaryEntry {
	summaries := make([]IssueSummaryEntry, 0, len(issues))
	for i := range issues {
		summaries = append(summaries, IssueSummaryEntry{
			Key:      issues[i].Key,
			Summary:  issues[i].Fields.Summary,
			Status:   statusEntry(issues[i].Fields.Status),
			Assignee: issues[i].Fields.Assignee.DisplayName,
			Updated:  issues[i].Fields.Updated,
		})
	}
	return summaries
}

func statusEntry(s jira.Status) StatusEntry {
	return StatusEntry{Name: s.Name, Category: s.StatusCategory.Key}
}

func authorType(u jira.User) string {
	switch u.AccountType {
	case "app":
		return "bot"
	case "atlassian", "customer":
		return "human"
	default:
		return ""
	}
}
