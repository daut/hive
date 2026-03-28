package cmd

import (
	"errors"
	"path/filepath"
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
}

func TestRunStartSuccess(t *testing.T) {
	resetCommandHooks(t)

	cmd, out := newTestCommand()
	ticketID := "hive-123"
	issue := &bdIssue{ID: ticketID, Title: "Generate a logo", Description: "Create a mark."}

	var gotCreatePath string
	var gotCreateBranch string
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

	output := out.String()
	for _, want := range []string{
		"Ticket: hive-123 - Generate a logo",
		"Status: moved to in_progress",
		"Worktree: /repo/.worktrees/hive-123",
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
