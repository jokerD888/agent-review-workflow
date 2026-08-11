# Agent Review Workflow

A portable, Git-first workflow for reviewing code written by Codex, Claude Code,
and OpenCode. It keeps AI work on a task branch, creates meaningful local
checkpoints, and makes the final accumulated diff easy to review in an IDE.

## What this installs

The installer adds a small managed block of personal workflow rules to each
agent's user-level instruction file. Existing instructions are preserved.

| Agent | User-level rules installed at |
| --- | --- |
| Codex | `~/.codex/AGENTS.md` (or `$CODEX_HOME/AGENTS.md`) |
| Claude Code | `~/.claude/CLAUDE.md` |
| OpenCode | `~/.config/opencode/AGENTS.md` |

The rules are guidance, not a security boundary. Keep approval prompts and Git
branch protection enabled for actions you do not want an agent to perform.

## Install on Windows

Clone the repository, then run:

```powershell
./install.ps1
```

To also add the shared project-level rules to an existing repository:

```powershell
./install.ps1 -ProjectPath C:\code\my-project -InstallOpenCodeReviewer
```

The project setup creates or updates these files without overwriting existing
content:

```text
my-project/
├── AGENTS.md                    # Shared by Codex and OpenCode
├── CLAUDE.md                    # Imports AGENTS.md for Claude Code
└── .opencode/agents/review.md   # Optional read-only OpenCode reviewer
```

Review and commit the generated project files before using `start-task`; the
helper deliberately refuses to create a task branch from a dirty working tree.

For a public fork, a one-line install is also supported:

```powershell
irm https://raw.githubusercontent.com/<owner>/agent-review-workflow/main/install.ps1 | iex
```

For a private repository, clone it first and run the local installer so GitHub
authentication is handled normally.

## Install on macOS or Linux

```bash
chmod +x install.sh
./install.sh
./install.sh --project ~/code/my-project --with-opencode-reviewer
```

## Daily workflow

Start a focused task branch from `main` (or provide another base branch):

```powershell
./scripts/start-task.ps1 -Name "fix login redirect"
```

Give the agent the business task. The installed rules tell it to avoid protected
branches and remote changes, keep scope narrow, run relevant checks, and create
local commits only at meaningful testable checkpoints.

When ready to review, inspect the whole branch before merging:

```powershell
./scripts/review.ps1 -OpenVSCode
```

The review helper shows the diff stat, commits, whitespace errors, and opens the
repository in VS Code. In VS Code, use Source Control and Source Control Graph
to inspect `main...HEAD`, individual commits, and file-level diffs.

## Why task branches, not daily branches?

A branch should describe a reviewable unit of product work. A date-based branch
often turns into a huge mixed diff that is hard to understand and hard to merge.
Small, low-risk tasks can still be reviewed together before merge; security,
public API, database, dependency, concurrency, and migration changes should be
reviewed separately.

## Updating or removing rules

Re-run the installer after pulling a newer version. It updates only blocks
between the `agent-review-workflow:begin` and `agent-review-workflow:end`
markers. Remove that marked block from a target file to uninstall it.

## Repository layout

```text
rules/global.md                  Shared user-level behavior
templates/AGENTS.md              Project-level shared template
templates/CLAUDE.md              Claude Code compatibility shim
templates/opencode-reviewer.md   Read-only OpenCode reviewer template
install.ps1 / install.sh         Idempotent installers
scripts/                         Start-task and review helpers
```

## License

[MIT](LICENSE)
