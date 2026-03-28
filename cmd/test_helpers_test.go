package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"
)

func newTestCommand() (*cobra.Command, *bytes.Buffer) {
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)
	return cmd, &out
}

func resetCommandHooks(t *testing.T) {
	t.Helper()

	originalFetchTicket := fetchTicketFn
	originalMoveToInProgress := moveToInProgressFn
	originalGitRepoRoot := gitRepoRootFn
	originalCreateWorktree := createWorktreeFn
	originalLaunchOpencode := launchOpencodeFn
	originalListWorktrees := listWorktreesFn
	originalIsBranchMerged := isBranchMergedFn
	originalRemoveWorktree := removeWorktreeFn
	originalDeleteBranch := deleteBranchFn
	originalExecCommand := execCommand
	originalLookPath := lookPath
	originalChangeDir := changeDir
	originalSyscallExec := syscallExec
	originalForceClean := forceClean

	t.Cleanup(func() {
		fetchTicketFn = originalFetchTicket
		moveToInProgressFn = originalMoveToInProgress
		gitRepoRootFn = originalGitRepoRoot
		createWorktreeFn = originalCreateWorktree
		launchOpencodeFn = originalLaunchOpencode
		listWorktreesFn = originalListWorktrees
		isBranchMergedFn = originalIsBranchMerged
		removeWorktreeFn = originalRemoveWorktree
		deleteBranchFn = originalDeleteBranch
		execCommand = originalExecCommand
		lookPath = originalLookPath
		changeDir = originalChangeDir
		syscallExec = originalSyscallExec
		forceClean = originalForceClean
	})
}

func stubExecCommand(output string, runErr error) func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=TestExecCommandHelper", "--", output}
		if runErr != nil {
			cmdArgs = append(cmdArgs, runErr.Error())
		}
		cmdArgs = append(cmdArgs, name)
		cmdArgs = append(cmdArgs, args...)
		cmd := exec.Command(os.Args[0], cmdArgs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
}

func TestExecCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	separator := 0
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}

	if separator == 0 || len(args) <= separator+1 {
		os.Exit(2)
	}

	output := args[separator+1]
	if len(args) > separator+2 && args[separator+2] != "" {
		_, _ = os.Stderr.WriteString(args[separator+2])
		os.Exit(1)
	}

	_, _ = os.Stdout.WriteString(output)
	os.Exit(0)
}
