# hive

CLI tool for orchestrating AI agent work sessions using [bd](https://github.com/bead-code/beads) tickets, git worktrees, and [opencode](https://opencode.ai).

## Install

```
go install github.com/daut/hive@latest
```

## Commands

### `hive start <ticket-id>`

Picks up a bd ticket and starts a work session:

1. Fetches ticket details from bd
2. Moves the ticket to `in_progress`
3. Creates a git worktree at `.worktrees/<ticket-id>`
4. Launches opencode with the ticket context and a planning prompt

### `hive list`

Lists all active worktree sessions under `.worktrees/`.

### `hive clean <ticket-id>`

Cleans up after a ticket's branch has been merged into main:

1. Verifies the branch is merged (use `--force` to skip)
2. Removes the git worktree
3. Deletes the local branch

## Requirements

- [bd](https://github.com/bead-code/beads) - issue tracker
- [opencode](https://opencode.ai) - AI coding assistant
- git
