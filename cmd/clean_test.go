package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMergedBaseRefPrefersOriginMain(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommandMap(t, map[string]commandResult{
		"git\x00rev-parse\x00--verify\x00--quiet\x00origin/main^{commit}": {output: "abc123\n"},
	})

	baseRef, err := mergedBaseRef()
	if err != nil {
		t.Fatalf("mergedBaseRef returned error: %v", err)
	}
	if baseRef != "origin/main" {
		t.Fatalf("mergedBaseRef = %q, want origin/main", baseRef)
	}
}

func TestMergedBaseRefFallsBackToMain(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommandMap(t, map[string]commandResult{
		"git\x00rev-parse\x00--verify\x00--quiet\x00origin/main^{commit}": {exitCode: 1},
		"git\x00rev-parse\x00--verify\x00--quiet\x00main^{commit}":        {output: "def456\n"},
	})

	baseRef, err := mergedBaseRef()
	if err != nil {
		t.Fatalf("mergedBaseRef returned error: %v", err)
	}
	if baseRef != "main" {
		t.Fatalf("mergedBaseRef = %q, want main", baseRef)
	}
}

func TestIsBranchMergedDetectsAncestorCommit(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommandMap(t, map[string]commandResult{
		"git\x00rev-parse\x00--verify\x00--quiet\x00origin/main^{commit}": {output: "abc123\n"},
		"git\x00merge-base\x00--is-ancestor\x00hive-123\x00origin/main":   {},
	})

	merged, err := isBranchMerged("hive-123")
	if err != nil {
		t.Fatalf("isBranchMerged returned error: %v", err)
	}
	if !merged {
		t.Fatal("expected branch to be detected as merged")
	}
}

func TestIsBranchMergedDetectsSquashMerge(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommandMap(t, map[string]commandResult{
		"git\x00rev-parse\x00--verify\x00--quiet\x00origin/main^{commit}": {output: "abc123\n"},
		"git\x00merge-base\x00--is-ancestor\x00hive-123\x00origin/main":   {exitCode: 1},
		"git\x00merge-tree\x00--write-tree\x00origin/main\x00hive-123":    {output: "tree123\n"},
		"git\x00rev-parse\x00origin/main^{tree}":                          {output: "tree123\n"},
	})

	merged, err := isBranchMerged("hive-123")
	if err != nil {
		t.Fatalf("isBranchMerged returned error: %v", err)
	}
	if !merged {
		t.Fatal("expected squash-merged branch to be detected as merged")
	}
}

func TestIsBranchMergedRejectsRemainingChangesAfterSquash(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommandMap(t, map[string]commandResult{
		"git\x00rev-parse\x00--verify\x00--quiet\x00origin/main^{commit}": {output: "abc123\n"},
		"git\x00merge-base\x00--is-ancestor\x00hive-123\x00origin/main":   {exitCode: 1},
		"git\x00merge-tree\x00--write-tree\x00origin/main\x00hive-123":    {output: "tree123\n"},
		"git\x00rev-parse\x00origin/main^{tree}":                          {output: "tree999\n"},
	})

	merged, err := isBranchMerged("hive-123")
	if err != nil {
		t.Fatalf("isBranchMerged returned error: %v", err)
	}
	if merged {
		t.Fatal("expected branch with remaining changes to be detected as not merged")
	}
}

func stubExecCommandMap(t *testing.T, results map[string]commandResult) func(string, ...string) *exec.Cmd {
	t.Helper()

	return func(name string, args ...string) *exec.Cmd {
		key := strings.Join(append([]string{name}, args...), "\x00")
		result, ok := results[key]
		if !ok {
			t.Fatalf("unexpected execCommand call: %q", key)
		}

		cmdArgs := []string{"-test.run=TestExecCommandHelper", "--", result.output, result.errText, strconv.Itoa(result.exitCode)}
		cmdArgs = append(cmdArgs, name)
		cmdArgs = append(cmdArgs, args...)

		cmd := exec.Command(os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
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
