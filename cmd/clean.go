package cmd

import (
	"fmt"
	"os"
	"os/exec"
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

func init() {
	cleanCmd.Flags().BoolVarP(&forceClean, "force", "f", false, "Force cleanup even if branch is not merged into main")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	ticketID := args[0]

	repoRoot, err := gitRepoRoot()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		return fmt.Errorf("worktree not found: %s", worktreePath)
	}

	branch := ticketID

	if !forceClean {
		merged, err := isBranchMerged(branch)
		if err != nil {
			return err
		}
		if !merged {
			return fmt.Errorf("branch %q is not merged into main; use --force to clean up anyway", branch)
		}
	}

	if err := removeWorktree(worktreePath); err != nil {
		return err
	}
	fmt.Printf("Removed worktree: %s\n", worktreePath)

	if err := deleteBranch(branch, forceClean); err != nil {
		return err
	}
	fmt.Printf("Deleted branch: %s\n", branch)

	return nil
}

func isBranchMerged(branch string) (bool, error) {
	out, err := exec.Command("git", "branch", "--merged", "main").Output()
	if err != nil {
		return false, fmt.Errorf("failed to check merged branches: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		name := strings.TrimSpace(line)
		if name == branch {
			return true, nil
		}
	}
	return false, nil
}

func removeWorktree(path string) error {
	cmd := exec.Command("bd", "worktree", "remove", path, "--force")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func deleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	cmd := exec.Command("git", "branch", flag, branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
