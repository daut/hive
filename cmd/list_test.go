package cmd

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseWorktreeListFiltersManagedWorktrees(t *testing.T) {
	repoRoot := "/repo"
	raw := strings.Join([]string{
		"worktree /repo",
		"HEAD abc123",
		"branch refs/heads/main",
		"",
		"worktree /repo/.worktrees/hive-101",
		"HEAD def456",
		"branch refs/heads/hive-101",
		"",
		"worktree /repo/.worktrees/hive-202",
		"HEAD fedcba",
		"branch refs/heads/feature/hive-202",
		"",
		"worktree /tmp/other",
		"HEAD 112233",
		"branch refs/heads/other",
		"",
	}, "\n")

	entries := parseWorktreeList(repoRoot, raw)
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	if entries[0] != (worktreeEntry{
		path:   filepath.Join(repoRoot, ".worktrees", "hive-101"),
		branch: "hive-101",
		ticket: "hive-101",
	}) {
		t.Fatalf("entries[0] = %#v", entries[0])
	}

	if entries[1] != (worktreeEntry{
		path:   filepath.Join(repoRoot, ".worktrees", "hive-202"),
		branch: "hive-202",
		ticket: "hive-202",
	}) {
		t.Fatalf("entries[1] = %#v", entries[1])
	}
}

func TestRunListPrintsNoSessionsMessage(t *testing.T) {
	resetCommandHooks(t)

	cmd, out := newTestCommand()
	gitRepoRootFn = func() (string, error) {
		return "/repo", nil
	}
	listWorktreesFn = func(repoRoot string) ([]worktreeEntry, error) {
		return nil, nil
	}

	if err := runList(cmd, nil); err != nil {
		t.Fatalf("runList returned error: %v", err)
	}

	if got := out.String(); got != "No active sessions.\n" {
		t.Fatalf("output = %q, want %q", got, "No active sessions.\n")
	}
}

func TestRunListPrintsWorktreeTable(t *testing.T) {
	resetCommandHooks(t)

	cmd, out := newTestCommand()
	entries := []worktreeEntry{
		{path: "/repo/.worktrees/hive-101", branch: "hive-101", ticket: "hive-101"},
		{path: "/repo/.worktrees/hive-202", branch: "hive-202", ticket: "hive-202"},
	}

	gitRepoRootFn = func() (string, error) {
		return "/repo", nil
	}
	listWorktreesFn = func(repoRoot string) ([]worktreeEntry, error) {
		if repoRoot != "/repo" {
			t.Fatalf("listWorktreesFn called with %q", repoRoot)
		}
		return entries, nil
	}

	if err := runList(cmd, nil); err != nil {
		t.Fatalf("runList returned error: %v", err)
	}

	output := out.String()
	for _, want := range []string{
		"TICKET",
		"BRANCH",
		"PATH",
		"hive-101",
		"/repo/.worktrees/hive-202",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}
