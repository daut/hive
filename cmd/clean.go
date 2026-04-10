package cmd

import (
	"errors"
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
	baseRef, err := mergedBaseRef()
	if err != nil {
		return false, err
	}

	merged, err := isAncestorCommit(branch, baseRef)
	if err != nil {
		return false, err
	}
	if merged {
		return true, nil
	}

	return mergeWouldBeNoop(baseRef, branch)
}

func mergedBaseRef() (string, error) {
	for _, ref := range []string{"origin/main", "main"} {
		exists, err := gitCommitRefExists(ref)
		if err != nil {
			return "", err
		}
		if exists {
			return ref, nil
		}
	}

	return "", fmt.Errorf("failed to resolve base branch: neither origin/main nor main exists")
}

func gitCommitRefExists(ref string) (bool, error) {
	cmd := execCommand("git", "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	if err := cmd.Run(); err != nil {
		if exitCode(err) == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to resolve git ref %q: %w", ref, err)
	}

	return true, nil
}

func isAncestorCommit(branch, baseRef string) (bool, error) {
	cmd := execCommand("git", "merge-base", "--is-ancestor", branch, baseRef)
	if err := cmd.Run(); err != nil {
		if exitCode(err) == 1 {
			return false, nil
		}
		return false, fmt.Errorf("failed to check whether %q is merged into %q: %w", branch, baseRef, err)
	}

	return true, nil
}

func mergeWouldBeNoop(baseRef, branch string) (bool, error) {
	mergedTree, err := mergedTreeOID(baseRef, branch)
	if err != nil {
		return false, err
	}

	baseTree, err := treeOID(baseRef)
	if err != nil {
		return false, err
	}

	return mergedTree == baseTree, nil
}

func mergedTreeOID(baseRef, branch string) (string, error) {
	out, err := execCommand("git", "merge-tree", "--write-tree", baseRef, branch).Output()
	if err != nil {
		return "", fmt.Errorf("failed to evaluate merge of %q into %q: %w", branch, baseRef, err)
	}

	return firstLine(string(out)), nil
}

func treeOID(ref string) (string, error) {
	out, err := execCommand("git", "rev-parse", ref+"^{tree}").Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve tree for %q: %w", ref, err)
	}

	return strings.TrimSpace(string(out)), nil
}

func firstLine(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return ""
}

func exitCode(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
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
