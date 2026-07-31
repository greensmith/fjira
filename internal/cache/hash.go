package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"time"
)

// JqlHash is the filename id for a search entry: the first 16 hex chars of
// SHA-256 over the exact JQL byte string. Deliberately no trimming,
// case-folding, or whitespace normalization — two "equivalent" JQLs hashing
// differently just costs a duplicate cache entry, while a buggy normalizer
// silently aliasing different queries would cost correctness. Cache readers
// implement the same rule.
func JqlHash(jql string) string {
	sum := sha256.Sum256([]byte(jql))
	return hex.EncodeToString(sum[:])[:16]
}

var expiresRegexp = regexp.MustCompile(`^([0-9]+)([dw])$`)

// ParseExpires parses the cache TTL: time.ParseDuration syntax plus day and
// week units, e.g. "3w", "10d", "36h".
func ParseExpires(s string) (time.Duration, error) {
	if m := expiresRegexp.FindStringSubmatch(s); m != nil {
		d, err := time.ParseDuration(m[1] + "h")
		if err != nil {
			return 0, err
		}
		if m[2] == "w" {
			return d * 24 * 7, nil
		}
		return d * 24, nil
	}
	return time.ParseDuration(s)
}
