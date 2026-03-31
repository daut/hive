package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

type bdIssue struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Dependents  []bdDependent `json:"dependents"`
}

type bdDependent struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Priority       int    `json:"priority"`
	IssueType      string `json:"issue_type"`
	DependencyType string `json:"dependency_type"`
}

var (
	fetchTicketFn      = fetchTicket
	moveToInProgressFn = moveToInProgress
	gitRepoRootFn      = gitRepoRoot
	createWorktreeFn   = createWorktree
	launchOpencodeFn   = launchOpencode
	execCommand        = exec.Command
	lookPath           = exec.LookPath
	changeDir          = os.Chdir
	syscallExec        = syscall.Exec
)

var startCmd = &cobra.Command{
	Use:   "start <ticket-id>",
	Short: "Start a new work session for a bd ticket",
	Long: `Start picks up a bd ticket, moves it to in_progress, creates a git worktree
under .worktrees/, and launches an opencode interactive session with the
ticket context and a planning prompt.`,
	Args: cobra.ExactArgs(1),
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	out := commandStdout(cmd)
	ticketID := args[0]

	issue, err := fetchTicketFn(ticketID)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Ticket: %s - %s\n", issue.ID, issue.Title)

	if err := moveToInProgressFn(ticketID); err != nil {
		return err
	}
	fmt.Fprintln(out, "Status: moved to in_progress")

	repoRoot, err := gitRepoRootFn()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if err := createWorktreeFn(worktreePath, ticketID); err != nil {
		return err
	}
	fmt.Fprintf(out, "Worktree: %s\n", worktreePath)

	prompt := buildPrompt(issue)
	fmt.Fprintln(out, "Launching opencode...")
	return launchOpencodeFn(worktreePath, prompt)
}

func fetchTicket(id string) (*bdIssue, error) {
	out, err := execCommand("bd", "show", id, "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticket %s: %w", id, err)
	}

	var issues []bdIssue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("failed to parse ticket JSON: %w", err)
	}
	if len(issues) != 1 {
		return nil, fmt.Errorf("expected exactly one ticket in bd show output for %s, got %d", id, len(issues))
	}

	return &issues[0], nil
}

func moveToInProgress(id string) error {
	cmd := execCommand("bd", "update", id, "--status", "in_progress")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitRepoRoot() (string, error) {
	out, err := execCommand("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func createWorktree(path, branch string) error {
	cmd := execCommand("bd", "worktree", "create", path, "--branch", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildPrompt(issue *bdIssue) string {
	var prompt strings.Builder
	fmt.Fprintf(&prompt, `Here is the ticket you'll be working on:

Title: %s
Description:
%s

`, issue.Title, issue.Description)

	children := directChildren(issue.Dependents)
	if len(children) > 0 {
		prompt.WriteString("Direct child tasks for reference:\n")
		for _, child := range children {
			fmt.Fprintf(&prompt, "- %s: %s [%s]\n", child.ID, child.Title, child.Status)
			if child.Description != "" {
				fmt.Fprintf(&prompt, "  Description: %s\n", child.Description)
			}
		}
		prompt.WriteString("\n")
	}

	prompt.WriteString(`Please create a plan for implementing this ticket. Break it down into clear,
actionable steps. Do not start implementing yet — just plan.`)

	return prompt.String()
}

func directChildren(dependents []bdDependent) []bdDependent {
	children := make([]bdDependent, 0, len(dependents))
	for _, dependent := range dependents {
		if dependent.DependencyType == "parent-child" {
			children = append(children, dependent)
		}
	}

	sort.Slice(children, func(i, j int) bool {
		return children[i].ID < children[j].ID
	})

	return children
}

func launchOpencode(dir, prompt string) error {
	binary, err := lookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH: %w", err)
	}

	if err := changeDir(dir); err != nil {
		return fmt.Errorf("failed to chdir to worktree: %w", err)
	}

	return syscallExec(binary, []string{"opencode", "--prompt", prompt}, os.Environ())
}

func commandStdout(cmd *cobra.Command) io.Writer {
	if cmd == nil {
		return os.Stdout
	}

	return cmd.OutOrStdout()
}
