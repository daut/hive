package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

type bdIssue struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

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
	ticketID := args[0]

	issue, err := fetchTicket(ticketID)
	if err != nil {
		return err
	}
	fmt.Printf("Ticket: %s — %s\n", issue.ID, issue.Title)

	if err := moveToInProgress(ticketID); err != nil {
		return err
	}
	fmt.Println("Status: moved to in_progress")

	repoRoot, err := gitRepoRoot()
	if err != nil {
		return err
	}

	worktreePath := filepath.Join(repoRoot, ".worktrees", ticketID)
	if err := createWorktree(worktreePath, ticketID); err != nil {
		return err
	}
	fmt.Printf("Worktree: %s\n", worktreePath)

	prompt := buildPrompt(issue)
	fmt.Println("Launching opencode...")
	return launchOpencode(worktreePath, prompt)
}

func fetchTicket(id string) (*bdIssue, error) {
	out, err := exec.Command("bd", "show", id, "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch ticket %s: %w", id, err)
	}

	var issue bdIssue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("failed to parse ticket JSON: %w", err)
	}
	return &issue, nil
}

func moveToInProgress(id string) error {
	cmd := exec.Command("bd", "update", id, "--status", "in_progress")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func createWorktree(path, branch string) error {
	cmd := exec.Command("bd", "worktree", "create", path, "--branch", branch)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func buildPrompt(issue *bdIssue) string {
	return fmt.Sprintf(`Here is the ticket you'll be working on:

Title: %s
Description:
%s

Please create a plan for implementing this ticket. Break it down into clear,
actionable steps. Do not start implementing yet — just plan.`, issue.Title, issue.Description)
}

func launchOpencode(dir, prompt string) error {
	binary, err := exec.LookPath("opencode")
	if err != nil {
		return fmt.Errorf("opencode not found in PATH: %w", err)
	}

	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("failed to chdir to worktree: %w", err)
	}

	return syscall.Exec(binary, []string{"opencode", "--prompt", prompt}, os.Environ())
}
