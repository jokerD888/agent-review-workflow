# Agent Review Workflow

A portable, Git-first workflow for reviewing code written by Codex, Claude Code,
and OpenCode. v2 tracks each unit of work as a task with a recorded base,
dependency chain, review history, and dedicated worktree.

> v2 is under active development. Its accepted architecture is documented in
> [the final solution design](docs/FINAL-SOLUTION-DESIGN.zh-CN.md). The original
> script workflow remains available as v1 compatibility only; do not mix its
> write commands with the v2 task ledger in one repository.

## v2 development quick start

Requirements: Git and Go 1.26+. From a Git repository with an initial `main`
commit, build ARW and create the first task:

```powershell
go build -o .\bin\arw.exe .\cmd\arw
.\bin\arw.exe setup
.\bin\arw.exe task start --id fix-login-redirect "修复登录跳转"
.\bin\arw.exe task list --format json
```

`arw setup` creates the local `arw/registry` branch. Task records live there,
not in product commits. `arw task start` makes a branch and a separate
worktree; it neither changes `main` nor pushes anything.

### When ARW is optional

ARW is for durable, reviewable delivery work—not every chat. Questions,
investigation, throwaway experiments, and disposable one-off tools stay outside
ARW by default. Say “这次不用 ARW”, “不建任务”, or “临时处理” to opt out for the
current conversation. If the intent is unclear, a configured agent should ask
once whether this is temporary work or a reviewable ARW task. Say “开始 ARW” or
“把这个纳入 ARW” to opt back in.

For a review, run `arw review prepare <task-id>`. It returns the exact recorded
base and task HEAD, commits, changed files, dirty-worktree status, dependency
state, test evidence, and risks. A child task is conditionally reviewable until
its parent has final approval. `arw review approve --confirm <task-id>` records
a human conclusion only; it cannot merge or push.

Record test evidence without executing arbitrary commands with `arw task
record-test --command "go test ./..." --result passed <task-id>`. Use `arw task
ready <task-id>` when implementation is ready to review. After a human-approved
task is merged outside ARW, record that fact with `arw task mark-merged --confirm
<task-id>`; `arw task abandon --confirm <task-id>` records an abandoned task
without deleting its branch or worktree.

The TypeScript extension is in `vscode-extension/`. Run `npm ci && npm run
package` there, then install the generated VSIX. It displays the task tree and
opens immutable Git-SHA-to-Git-SHA native VS Code diffs, so the visible diff is
not accidentally based on whichever branch is currently open.

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

The installer also installs the short `arw` command into your user PATH. Open a
new terminal after installation, then run `arw help`.

## v2 release installation

Install only versioned Release assets (never a mutable `main` script). The
installers download the matching binary, verify `checksums.txt`, add its
user-local `bin` directory to PATH, and can install the VSIX. Each release also
has GitHub build provenance attached by the release workflow; installers verify
the checksum manifest but do not currently verify attestations locally:

```powershell
irm https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/installers/install.ps1 -OutFile install-arw.ps1
.\install-arw.ps1 -InstallExtension -ConfigureAgents
```

```bash
curl -fsSLO https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/installers/install.sh
sh install.sh --with-extension --configure-agents
```

Use `--version vX.Y.Z` to pin a release; use `--force` only to replace an
existing ARW installation. `arw update` is intentionally not available until a
release has been verified; rerun the versioned installer instead.

`-ConfigureAgents` on Windows and `--configure-agents` on Linux add the local,
typed `arw-mcp` service to Codex, Claude Code, and OpenCode. Restart those
applications after installing. Linux OpenCode setup uses Python 3's standard
JSON library to preserve existing user configuration.

## v1 installation (compatibility only)

For a quick user-level installation on Windows, run:

```powershell
irm https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/install.ps1 | iex
```

For macOS or Linux, run:

```bash
curl -fsSL https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/install.sh | sh
```

To configure a specific project, clone the repository and pass its local path
to the installer as shown above.

## Install on macOS or Linux

```bash
chmod +x install.sh
./install.sh
./install.sh --project ~/code/my-project --with-opencode-reviewer
```

## Daily workflow

Start a focused task branch from `main` (or provide another base branch):

```powershell
arw start "fix login redirect"
```

Give the agent the business task. The installed rules tell it to avoid protected
branches and remote changes, keep scope narrow, run relevant checks, and create
local commits only at meaningful testable checkpoints.

When ready to review, inspect the whole branch before merging:

```powershell
arw review -OpenVSCode
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
install.ps1 / install.sh         Idempotent installers and CLI bootstrap
arw.ps1 / arw.sh                 Cross-platform `arw` command dispatchers
scripts/                         Start-task and review helpers
cmd/arw/                         v2 Go CLI
cmd/arw-mcp/                     v2 stdio MCP server
internal/                        v2 Git, ledger, task, review, worktree logic
schemas/                         JSON contracts for tasks and review snapshots
vscode-extension/                v2 VS Code task and diff UI
integrations/                    Natural-language rules for each supported agent
```

## License

[MIT](LICENSE)
