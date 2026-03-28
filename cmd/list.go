package cmd

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List active worktree sessions",
	Args:  cobra.NoArgs,
	RunE:  runList,
}

var listWorktreesFn = listWorktrees

func init() {
	rootCmd.AddCommand(listCmd)
}

type worktreeEntry struct {
	path   string
	branch string
	ticket string
}

func runList(cmd *cobra.Command, args []string) error {
	out := commandStdout(cmd)

	repoRoot, err := gitRepoRootFn()
	if err != nil {
		return err
	}

	entries, err := listWorktreesFn(repoRoot)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Fprintln(out, "No active sessions.")
		return nil
	}

	fmt.Fprintf(out, "%-15s %-20s %s\n", "TICKET", "BRANCH", "PATH")
	for _, e := range entries {
		fmt.Fprintf(out, "%-15s %-20s %s\n", e.ticket, e.branch, e.path)
	}
	return nil
}

func listWorktrees(repoRoot string) ([]worktreeEntry, error) {
	out, err := execCommand("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(repoRoot, string(out)), nil
}

func parseWorktreeList(repoRoot, raw string) []worktreeEntry {
	worktreesDir := filepath.Join(repoRoot, ".worktrees")
	var entries []worktreeEntry
	var current worktreeEntry

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = worktreeEntry{path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch "):
			ref := strings.TrimPrefix(line, "branch ")
			current.branch = filepath.Base(ref)
		case line == "":
			if strings.HasPrefix(current.path, worktreesDir) {
				current.ticket = filepath.Base(current.path)
				entries = append(entries, current)
			}
			current = worktreeEntry{}
		}
	}

	return entries
}
