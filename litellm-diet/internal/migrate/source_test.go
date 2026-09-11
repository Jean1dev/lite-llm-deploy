package migrate

import (
	"strings"
	"testing"
)

func TestSourceQueryMatchesAvailableColumns(t *testing.T) {
	cases := []struct {
		name       string
		flags      sourceFlags
		want       []string
		wantAbsent []string
	}{
		{
			name:       "production litellm without deleted",
			flags:      sourceFlags{lastActive: true},
			want:       []string{"COALESCE(last_active, updated_at)", "false"},
			wantAbsent: []string{"deleted"},
		},
		{
			name:  "full source schema",
			flags: sourceFlags{deleted: true, lastActive: true},
			want:  []string{"COALESCE(last_active, updated_at)", "COALESCE(deleted, false)"},
		},
		{
			name:       "minimal source schema",
			flags:      sourceFlags{},
			want:       []string{"updated_at", "false"},
			wantAbsent: []string{"deleted", "last_active"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			q := sourceQuery(c.flags)
			for _, fragment := range c.want {
				if !strings.Contains(q, fragment) {
					t.Fatalf("query missing %q:\n%s", fragment, q)
				}
			}
			for _, fragment := range c.wantAbsent {
				if strings.Contains(q, fragment) {
					t.Fatalf("query should omit %q:\n%s", fragment, q)
				}
			}
		})
	}
}
