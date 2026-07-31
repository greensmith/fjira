package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_JqlHash(t *testing.T) {
	// sha256("project = TEST") = 73a7de6bd51c391d... — pinned so the hash rule
	// never drifts; cache readers hash the same bytes and must get this name.
	assert.Equal(t, "73a7de6bd51c391d", JqlHash("project = TEST"))
	// no normalization by design: whitespace/case variants are distinct ids
	assert.NotEqual(t, JqlHash("project = TEST"), JqlHash("project =  TEST"))
	assert.NotEqual(t, JqlHash("project = TEST"), JqlHash("PROJECT = TEST"))
	assert.Len(t, JqlHash(""), 16)
}

func Test_ParseExpires(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"3w", 3 * 7 * 24 * time.Hour},
		{"10d", 10 * 24 * time.Hour},
		{"36h", 36 * time.Hour},
		{"90m", 90 * time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseExpires(tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
	_, err := ParseExpires("soon")
	assert.Error(t, err)
}
