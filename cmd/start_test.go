package cmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildPromptIncludesTicketDetails(t *testing.T) {
	issue := &bdIssue{
		Title:       "Generate a logo",
		Description: "Create a simple hive mark for the CLI README.",
	}

	prompt := buildPrompt(issue)

	for _, want := range []string{
		"Title: Generate a logo",
		"Create a simple hive mark for the CLI README.",
		"Please create a plan for implementing this ticket.",
		"Do not start implementing yet",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}
	if strings.Contains(prompt, "Direct child tasks for reference:") {
		t.Fatalf("prompt %q unexpectedly contains child task section", prompt)
	}
}

func TestFetchTicketParsesBeadsShowJSONArray(t *testing.T) {
	resetCommandHooks(t)

	want := &bdIssue{
		ID:          "markan-frd",
		Title:       "Phase 2 manual LinkedIn publishing rollout",
		Description: "Track implementation work.",
		Dependents: []bdDependent{{
			ID:             "markan-frd.1",
			Title:          "LinkedIn OAuth Integration",
			Description:    "Implement OAuth setup.",
			Status:         "open",
			DependencyType: "parent-child",
		}},
	}
	raw, err := json.Marshal([]bdIssue{*want})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	execCommand = stubExecCommand(string(raw), nil)

	got, err := fetchTicket(want.ID)
	if err != nil {
		t.Fatalf("fetchTicket returned error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fetchTicket returned %+v, want %+v", *got, *want)
	}
}

func TestBuildPromptIncludesDirectChildTasksForReference(t *testing.T) {
	issue := &bdIssue{
		Title:       "Phase 2 manual LinkedIn publishing rollout",
		Description: "Track implementation work.",
		Dependents: []bdDependent{
			{
				ID:             "markan-frd.2",
				Title:          "LinkedIn Posting Client",
				Description:    "Build posting client.",
				Status:         "open",
				DependencyType: "parent-child",
			},
			{
				ID:             "markan-frd-related",
				Title:          "Something related",
				Status:         "open",
				DependencyType: "related",
			},
			{
				ID:             "markan-frd.1",
				Title:          "LinkedIn OAuth Integration",
				Status:         "in_progress",
				DependencyType: "parent-child",
			},
		},
	}

	prompt := buildPrompt(issue)

	for _, want := range []string{
		"Direct child tasks for reference:",
		"- markan-frd.1: LinkedIn OAuth Integration [in_progress]",
		"- markan-frd.2: LinkedIn Posting Client [open]",
		"  Description: Build posting client.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt %q does not contain %q", prompt, want)
		}
	}

	if strings.Contains(prompt, "markan-frd-related") {
		t.Fatalf("prompt %q unexpectedly contains non-child dependent", prompt)
	}

	if strings.Index(prompt, "markan-frd.1") > strings.Index(prompt, "markan-frd.2") {
		t.Fatalf("prompt %q did not sort child tasks by id", prompt)
	}
}

func TestFetchTicketRejectsUnexpectedResultCount(t *testing.T) {
	resetCommandHooks(t)

	execCommand = stubExecCommand("[]", nil)

	_, err := fetchTicket("markan-frd")
	if err == nil || !strings.Contains(err.Error(), "expected exactly one ticket") {
		t.Fatalf("fetchTicket error = %v, want exactly one ticket error", err)
	}
}

func TestRunStartSuccess(t *testing.T) {
	resetCommandHooks(t)

	cmd, out := newTestCommand()
	ticketID := "hive-123"
	issue := &bdIssue{ID: ticketID, Title: "Generate a logo", Description: "Create a mark."}

	var gotCreatePath string
	var gotCreateBranch string
	var gotLinkRepoRoot string
	var gotLinkWorktreePath string
	var gotLaunchDir string
	var gotLaunchPrompt string

	fetchTicketFn = func(id string) (*bdIssue, error) {
		if id != ticketID {
			t.Fatalf("fetchTicketFn called with %q", id)
		}
		return issue, nil
	}
	moveToInProgressFn = func(id string) error {
		if id != ticketID {
			t.Fatalf("moveToInProgressFn called with %q", id)
		}
		return nil
	}
	gitRepoRootFn = func() (string, error) {
		return "/repo", nil
	}
	createWorktreeFn = func(path, branch string) error {
		gotCreatePath = path
		gotCreateBranch = branch
		return nil
	}
	linkUntrackedFilesFn = func(repoRoot, worktreePath string) (int, error) {
		gotLinkRepoRoot = repoRoot
		gotLinkWorktreePath = worktreePath
		return 2, nil
	}
	launchOpencodeFn = func(dir, prompt string) error {
		gotLaunchDir = dir
		gotLaunchPrompt = prompt
		return nil
	}

	if err := runStart(cmd, []string{ticketID}); err != nil {
		t.Fatalf("runStart returned error: %v", err)
	}

	wantPath := filepath.Join("/repo", ".worktrees", ticketID)
	if gotCreatePath != wantPath {
		t.Fatalf("createWorktreeFn path = %q, want %q", gotCreatePath, wantPath)
	}
	if gotCreateBranch != ticketID {
		t.Fatalf("createWorktreeFn branch = %q, want %q", gotCreateBranch, ticketID)
	}
	if gotLaunchDir != wantPath {
		t.Fatalf("launchOpencodeFn dir = %q, want %q", gotLaunchDir, wantPath)
	}
	if gotLaunchPrompt != buildPrompt(issue) {
		t.Fatalf("launchOpencodeFn prompt = %q, want %q", gotLaunchPrompt, buildPrompt(issue))
	}

	if gotLinkRepoRoot != "/repo" {
		t.Fatalf("linkUntrackedFilesFn repoRoot = %q, want %q", gotLinkRepoRoot, "/repo")
	}
	if gotLinkWorktreePath != wantPath {
		t.Fatalf("linkUntrackedFilesFn worktreePath = %q, want %q", gotLinkWorktreePath, wantPath)
	}

	output := out.String()
	for _, want := range []string{
		"Ticket: hive-123 - Generate a logo",
		"Status: moved to in_progress",
		"Worktree: /repo/.worktrees/hive-123",
		"Linked 2 untracked files/dirs",
		"Launching opencode...",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output %q does not contain %q", output, want)
		}
	}
}

func TestRunStartStopsOnMoveError(t *testing.T) {
	resetCommandHooks(t)

	cmd, _ := newTestCommand()
	ticketID := "hive-123"
	moveErr := errors.New("bd update failed")
	gitRepoRootCalled := false

	fetchTicketFn = func(id string) (*bdIssue, error) {
		return &bdIssue{ID: id, Title: "Generate a logo"}, nil
	}
	moveToInProgressFn = func(id string) error {
		return moveErr
	}
	gitRepoRootFn = func() (string, error) {
		gitRepoRootCalled = true
		return "/repo", nil
	}

	err := runStart(cmd, []string{ticketID})
	if !errors.Is(err, moveErr) {
		t.Fatalf("runStart error = %v, want %v", err, moveErr)
	}
	if gitRepoRootCalled {
		t.Fatal("gitRepoRootFn should not be called after move failure")
	}
}

func TestLinkUntrackedFilesSymlinksTopLevelFiles(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".worktrees", "hive-123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	envFile := filepath.Join(repoRoot, ".env")
	if err := os.WriteFile(envFile, []byte("KEY=VAL"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gitHasTrackedFilesFn = func(root, path string) (bool, error) {
		return false, nil
	}

	linked, err := linkUntrackedFiles(repoRoot, worktreePath)
	if err != nil {
		t.Fatalf("linkUntrackedFiles returned error: %v", err)
	}
	if linked != 1 {
		t.Fatalf("linked = %d, want 1", linked)
	}

	link := filepath.Join(worktreePath, ".env")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != envFile {
		t.Fatalf("symlink target = %q, want %q", target, envFile)
	}
}

func TestLinkUntrackedFilesSymlinksEntireUntrackedDirs(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".worktrees", "hive-123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	nodeModules := filepath.Join(repoRoot, "node_modules", "pkg")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "index.js"), []byte("module.exports"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gitHasTrackedFilesFn = func(root, path string) (bool, error) {
		return false, nil
	}

	linked, err := linkUntrackedFiles(repoRoot, worktreePath)
	if err != nil {
		t.Fatalf("linkUntrackedFiles returned error: %v", err)
	}
	if linked != 1 {
		t.Fatalf("linked = %d, want 1", linked)
	}

	target, err := os.Readlink(filepath.Join(worktreePath, "node_modules"))
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != filepath.Join(repoRoot, "node_modules") {
		t.Fatalf("symlink target = %q, want %q", target, filepath.Join(repoRoot, "node_modules"))
	}

	content, err := os.ReadFile(filepath.Join(worktreePath, "node_modules", "pkg", "index.js"))
	if err != nil {
		t.Fatalf("ReadFile through symlink: %v", err)
	}
	if string(content) != "module.exports" {
		t.Fatalf("content = %q, want %q", content, "module.exports")
	}
}

func TestLinkUntrackedFilesSymlinksIndividualFilesInMixedDirs(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".worktrees", "hive-123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	dataDir := filepath.Join(repoRoot, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	dbFile := filepath.Join(dataDir, "app.db")
	if err := os.WriteFile(dbFile, []byte("sqlite"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gitHasTrackedFilesFn = func(root, path string) (bool, error) {
		if path == "data" {
			return true, nil
		}
		return false, nil
	}
	gitUntrackedFilesFn = func(root, dir string) ([]string, error) {
		return []string{"data/app.db"}, nil
	}

	linked, err := linkUntrackedFiles(repoRoot, worktreePath)
	if err != nil {
		t.Fatalf("linkUntrackedFiles returned error: %v", err)
	}
	if linked != 1 {
		t.Fatalf("linked = %d, want 1", linked)
	}

	linkPath := filepath.Join(worktreePath, "data", "app.db")
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Readlink: %v", err)
	}
	if target != dbFile {
		t.Fatalf("symlink target = %q, want %q", target, dbFile)
	}
}

func TestLinkUntrackedFilesSkipsGitAndWorktrees(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".worktrees", "hive-123")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	gitHasTrackedFilesFn = func(root, path string) (bool, error) {
		t.Fatalf("should not be called for %q", path)
		return false, nil
	}

	linked, err := linkUntrackedFiles(repoRoot, worktreePath)
	if err != nil {
		t.Fatalf("linkUntrackedFiles returned error: %v", err)
	}
	if linked != 0 {
		t.Fatalf("linked = %d, want 0", linked)
	}
}

func TestLinkUntrackedFilesSkipsExistingTargets(t *testing.T) {
	resetCommandHooks(t)

	repoRoot := t.TempDir()
	worktreePath := filepath.Join(repoRoot, ".worktrees", "hive-123")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	envFile := filepath.Join(repoRoot, ".env")
	if err := os.WriteFile(envFile, []byte("KEY=VAL"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	existingFile := filepath.Join(worktreePath, ".env")
	if err := os.WriteFile(existingFile, []byte("EXISTING"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	gitHasTrackedFilesFn = func(root, path string) (bool, error) {
		return false, nil
	}

	linked, err := linkUntrackedFiles(repoRoot, worktreePath)
	if err != nil {
		t.Fatalf("linkUntrackedFiles returned error: %v", err)
	}
	if linked != 0 {
		t.Fatalf("linked = %d, want 0", linked)
	}

	content, err := os.ReadFile(existingFile)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(content) != "EXISTING" {
		t.Fatalf("content = %q, want %q", content, "EXISTING")
	}
}
