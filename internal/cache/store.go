// Package cache writes fjira's on-disk issue cache for external readers.
//
// fjira is the sole owner of the cache directory: it creates it, writes every
// file, and reaps expired entries. Readers are strictly read-only and treat
// anything missing or unparseable as a cache miss. The layout is
//
//	<root>/<workspace>/<kind>/<id>.<date>.yaml
//
// where kind is issue/, search/ or children/, id is the issue key verbatim
// (or a JQL hash, see JqlHash), and date is the local fetch day. Each file is
// a YAML document beginning with the Envelope fields; readers ignore unknown
// fields and skip files with a newer schema V. One entity keeps at most one
// file per kind: a refresh writes the new date then removes the old
// (replace-in-place), and every write is temp-file + rename so readers never
// observe a torn file.
package cache

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// SchemaVersion is the envelope `v:`. Additive changes keep it; breaking
// changes (rename, retype, resemantic) bump it, and readers treat a newer
// version as a miss.
const SchemaVersion = 1

const defaultWorkspaceName = "default"

// Envelope is the common header every cache file starts with.
type Envelope struct {
	V         int    `json:"v" yaml:"v"`
	Kind      string `json:"kind" yaml:"kind"`
	Workspace string `json:"workspace" yaml:"workspace"`
	// FetchedAt is the precise fetch time, RFC3339 with local offset. Its
	// local date equals the <date> in the filename.
	FetchedAt string `json:"fetchedAt" yaml:"fetchedAt"`
}

type StatusEntry struct {
	Name string `json:"name" yaml:"name"`
	// Category is the workflow-independent jira statusCategory key:
	// new | indeterminate | done.
	Category string `json:"category" yaml:"category"`
}

type CommentEntry struct {
	Author string `json:"author" yaml:"author"`
	// AuthorType is "human" or "bot", or "" when not derivable (jira server
	// doesn't expose accountType).
	AuthorType string `json:"authorType" yaml:"authorType"`
	Created    string `json:"created" yaml:"created"`
	Body       string `json:"body" yaml:"body"`
}

// IssueEntry is the detail tier: current state plus full comments.
type IssueEntry struct {
	Envelope `yaml:",inline"`
	Key      string         `json:"key" yaml:"key"`
	Summary  string         `json:"summary" yaml:"summary"`
	Url      string         `json:"url" yaml:"url"`
	Type     string         `json:"type" yaml:"type"`
	Status   StatusEntry    `json:"status" yaml:"status"`
	Assignee string         `json:"assignee" yaml:"assignee"`
	Reporter string         `json:"reporter" yaml:"reporter"`
	Parent   string         `json:"parent" yaml:"parent"`
	Updated  string         `json:"updated" yaml:"updated"`
	Comments []CommentEntry `json:"comments" yaml:"comments"`
}

// IssueSummaryEntry is the summary tier used by search and children lists.
type IssueSummaryEntry struct {
	Key      string      `json:"key" yaml:"key"`
	Summary  string      `json:"summary" yaml:"summary"`
	Status   StatusEntry `json:"status" yaml:"status"`
	Assignee string      `json:"assignee" yaml:"assignee"`
	Updated  string      `json:"updated" yaml:"updated"`
}

type SearchEntry struct {
	Envelope `yaml:",inline"`
	// Jql is the verbatim query, kept for humans; the filename id is its hash.
	Jql    string              `json:"jql" yaml:"jql"`
	Issues []IssueSummaryEntry `json:"issues" yaml:"issues"`
}

type ChildrenEntry struct {
	Envelope `yaml:",inline"`
	Parent   string              `json:"parent" yaml:"parent"`
	Issues   []IssueSummaryEntry `json:"issues" yaml:"issues"`
}

// NewEnvelope returns an envelope stamped with the current time, for entries
// that are emitted directly (e.g. headless JSON on stdout) rather than
// written by a Store. An empty workspace resolves to "default", matching
// NewStore.
func NewEnvelope(kind string, workspace string) Envelope {
	if workspace == "" {
		workspace = defaultWorkspaceName
	}
	return Envelope{
		V:         SchemaVersion,
		Kind:      kind,
		Workspace: workspace,
		FetchedAt: time.Now().Format(time.RFC3339),
	}
}

// Store writes cache files for one workspace under one cache root.
type Store struct {
	root      string
	workspace string
	now       func() time.Time
}

func NewStore(root string, workspace string) *Store {
	if workspace == "" {
		workspace = defaultWorkspaceName
	}
	return &Store{root: root, workspace: workspace, now: time.Now}
}

func (s *Store) WriteIssue(entry IssueEntry) error {
	return s.write("issue", entry.Key, &entry.Envelope, &entry)
}

func (s *Store) WriteSearch(entry SearchEntry) error {
	return s.write("search", JqlHash(entry.Jql), &entry.Envelope, &entry)
}

func (s *Store) WriteChildren(entry ChildrenEntry) error {
	return s.write("children", entry.Parent, &entry.Envelope, &entry)
}

// write stamps the envelope, writes doc to a same-directory temp file,
// renames it onto <id>.<date>.yaml, then removes any older-dated file for the
// same id. The rename is what makes a new file appear atomically to readers;
// the trailing unlink is the replace-in-place rule (readers still take the
// lexically-newest date if a crash leaves two).
func (s *Store) write(kind string, id string, envelope *Envelope, doc any) error {
	now := s.now()
	envelope.V = SchemaVersion
	envelope.Kind = kind
	envelope.Workspace = s.workspace
	envelope.FetchedAt = now.Format(time.RFC3339)
	data, err := yaml.Marshal(doc)
	if err != nil {
		return err
	}
	dir := filepath.Join(s.root, s.workspace, kind)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	final := fmt.Sprintf("%s.%s.yaml", id, now.Format("2006-01-02"))
	tmp := filepath.Join(dir, "."+final+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, final)); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	older, _ := filepath.Glob(filepath.Join(dir, id+".*.yaml"))
	for _, f := range older {
		if filepath.Base(f) != final {
			_ = os.Remove(f)
		}
	}
	return nil
}

// Reap removes cache files (including orphaned temp files) whose mtime is
// older than ttl. Only the writer ever deletes from the cache; readers render
// stale entries with their age rather than hiding them, so reaping is purely
// disk hygiene. Best-effort: errors are ignored.
func (s *Store) Reap(ttl time.Duration) {
	cutoff := s.now().Add(-ttl)
	for _, kind := range []string{"issue", "search", "children", "lookup"} {
		files, _ := filepath.Glob(filepath.Join(s.root, s.workspace, kind, "*"))
		for _, f := range files {
			if stat, err := os.Stat(f); err == nil && !stat.IsDir() && stat.ModTime().Before(cutoff) {
				_ = os.Remove(f)
			}
		}
	}
}
