package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchInMergedOutput(t *testing.T) {
	raw := strings.Join([]string{
		"* main",
		"  hive-123",
		"  release",
	}, "\n")

	if !branchInMergedOutput("hive-123", raw) {
		t.Fatal("expected branch to be detected as merged")
	}
	if branchInMergedOutput("hive-999", raw) {
		t.Fatal("expected branch to be detected as not merged")
	}
}

func TestRunCleanRejectsUnmergedBranch(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	ticketID := "hive-123"
	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmd, _ := newTestCommand()
	forceClean = false
	mergeCheckCalled := false
	removeCalled := false
	deleteCalled := false

	gitRepoRootFn = func() (string, error) {
		return repoRoot, nil
	}
	isBranchMergedFn = func(branch string) (bool, error) {
		mergeCheckCalled = true
		if branch != ticketID {
			t.Fatalf("isBranchMergedFn called with %q", branch)
		}
		return false, nil
	}
	removeWorktreeFn = func(path string) error {
		removeCalled = true
		return nil
	}
	deleteBranchFn = func(branch string, force bool) error {
		deleteCalled = true
		return nil
	}

	err := runClean(cmd, []string{ticketID})
	if err == nil || !strings.Contains(err.Error(), "not merged into main") {
		t.Fatalf("runClean error = %v, want not merged error", err)
	}
	if !mergeCheckCalled {
		t.Fatal("expected merge check to be called")
	}
	if removeCalled {
		t.Fatal("removeWorktreeFn should not be called for unmerged branch")
	}
	if deleteCalled {
		t.Fatal("deleteBranchFn should not be called for unmerged branch")
	}
}

func TestRunCleanForceRemovesWorktreeAndBranch(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	ticketID := "hive-123"
	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	cmd, out := newTestCommand()
	forceClean = true
	mergeCheckCalled := false
	removedPath := ""
	deletedBranch := ""
	deletedForce := false

	gitRepoRootFn = func() (string, error) {
		return repoRoot, nil
	}
	isBranchMergedFn = func(branch string) (bool, error) {
		mergeCheckCalled = true
		return false, nil
	}
	removeWorktreeFn = func(path string) error {
		removedPath = path
		return nil
	}
	deleteBranchFn = func(branch string, force bool) error {
		deletedBranch = branch
		deletedForce = force
		return nil
	}

	if err := runClean(cmd, []string{ticketID}); err != nil {
		t.Fatalf("runClean returned error: %v", err)
	}

	if mergeCheckCalled {
		t.Fatal("merge check should be skipped when forceClean is true")
	}
	if removedPath != worktreePath {
		t.Fatalf("removeWorktreeFn path = %q, want %q", removedPath, worktreePath)
	}
	if deletedBranch != ticketID {
		t.Fatalf("deleteBranchFn branch = %q, want %q", deletedBranch, ticketID)
	}
	if !deletedForce {
		t.Fatal("deleteBranchFn force = false, want true")
	}

	output := out.String()
	for _, want := range []string{
		"Removed worktree: " + worktreePath,
		"Deleted branch: " + ticketID,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}
