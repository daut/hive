package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var forceClean bool

var cleanCmd = &cobra.Command{
	Use:   "clean <ticket-id>",
	Short: "Clean up a worktree and branch after a ticket's branch is merged",
	Args:  cobra.ExactArgs(1),
	RunE:  runClean,
}

var (
	isBranchMergedFn = isBranchMerged
	removeWorktreeFn = removeWorktree
	deleteBranchFn   = deleteBranch
)

func init() {
	cleanCmd.Flags().BoolVarP(&forceClean, "force", "f", false, "Force cleanup even if branch is not merged into main")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	out := commandStdout(cmd)
	ticketID := args[0]

	repoRoot, err := gitRepoRootFn()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree not found: %s", worktreePath)
	}

	branch := ticketID

	if !forceClean {
		merged, err := isBranchMergedFn(branch)
		if err != nil {
			return err
		}
		if !merged {
			return fmt.Errorf("branch %q is not merged into main; use --force to clean up anyway", branch)
		}
	}

	if err := removeWorktreeFn(worktreePath); err != nil {
		return err
	}
	fmt.Fprintf(out, "Removed worktree: %s\n", worktreePath)

	if err := deleteBranchFn(branch, forceClean); err != nil {
		return err
	}
	fmt.Fprintf(out, "Deleted branch: %s\n", branch)

	return nil
}

func isBranchMerged(branch string) (bool, error) {
	out, err := execCommand("git", "branch", "--merged", "main").Output()
	if err != nil {
		return false, fmt.Errorf("failed to check merged branches: %w", err)
	}

	return branchInMergedOutput(branch, string(out)), nil
}

func branchInMergedOutput(branch, raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		name := strings.TrimSpace(line)
		if name == branch {
			return true
		}
	}
	return false
}

func removeWorktree(path string) error {
	cmd := execCommand("bd", "worktree", "remove", path, "--force")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := execCommand("git", "branch", flag, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
