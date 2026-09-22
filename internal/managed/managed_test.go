package managed_test

import (
	"strings"
	"testing"

	"github.com/discobox-ai/repostd/internal/managed"
)

func TestMergeLocalCarriesRepoBlocksIntoTheCanonicalFile(t *testing.T) {
	canonical := "a\n  # repostd:local enable\n  # repostd:end\nb\n  # repostd:local rules\n  # repostd:end\n"
	current := "old\n  # repostd:local enable\n    - gocritic\n  # repostd:end\n  # repostd:local gone\n    - x\n  # repostd:end\n"

	got := string(managed.MergeLocal([]byte(canonical), []byte(current)))
	want := "a\n  # repostd:local enable\n    - gocritic\n  # repostd:end\nb\n  # repostd:local rules\n  # repostd:end\n"
	if got != want {
		t.Errorf("MergeLocal =\n%s\nwant\n%s", got, want)
	}
}

func TestMergeLocalOfTheCanonicalFileIsTheCanonicalFile(t *testing.T) {
	canonical := "x\n# repostd:local a\n# repostd:end\n"
	if got := string(managed.MergeLocal([]byte(canonical), nil)); got != canonical {
		t.Errorf("MergeLocal = %q, want %q", got, canonical)
	}
	if blocks := managed.LocalBlocks([]byte(canonical)); strings.Join(mapKeys(blocks), ",") != "a" || blocks["a"] != "" {
		t.Errorf("LocalBlocks = %v, want one empty block a", blocks)
	}
}

func mapKeys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
