# Agent Review Workflow

A portable, Git-first workflow for reviewing code written by Codex, Claude Code,
and OpenCode. v2 tracks each unit of work as a task with a recorded base,
dependency chain, review history, and dedicated worktree.

> v2 is under active development. Its accepted architecture is documented in
> [the final solution design](docs/FINAL-SOLUTION-DESIGN.zh-CN.md). This
> repository supports only the Go-based v2 workflow.

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

For a review, run `arw review prepare <task-id>`. It resolves and returns the
exact current base and task HEAD, commits, changed files, dirty-worktree status, dependency
state, approval validity, and risks. A child task can still be inspected before
its parent is approved, but ARW refuses its final approval until the dependency
is clear. After the user explicitly says the task passed,
a configured agent can call `workflow_approve_task`; this records the user's
conclusion for the exact base and HEAD they reviewed, not the agent's own
judgment. If either SHA has changed, approval is refused.

Use `arw task ready <task-id>` when implementation is ready to review. A separate
explicit user request can call `workflow_merge_task` or `arw task merge
--confirm <task-id>` to fast-forward the approved task into its recorded
parent/base branch. It refuses moved targets, dirty worktrees, non-fast-forward
integration, and conflicts; it never pushes. `arw task abandon --confirm`
records an abandoned task without deleting its branch or worktree. External
merges are intentionally not recorded by an unchecked command: reconcile them
manually for now, or use ARW's validated local merge.

ARW does not provide a custom editor extension. Use VS Code Source Control to
inspect the task branch and its recorded base ref; use GitLens when you need an
arbitrary-ref comparison. ARW's responsibility is to return and validate the
exact base/head SHA pair, not to replace mature Git diff interfaces.

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

## v2 release installation

Install only versioned Release assets (never a mutable `main` script). The
installers download the matching binaries, verify `checksums.txt`, and add their
user-local `bin` directory to PATH. Each release also has GitHub build
provenance attached by the release workflow; installers verify the checksum
manifest but do not currently verify attestations locally:

```powershell
irm https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/installers/install.ps1 -OutFile install-arw.ps1
.\install-arw.ps1 -ConfigureAgents
```

```bash
curl -fsSLO https://raw.githubusercontent.com/jokerD888/agent-review-workflow/main/installers/install.sh
sh install.sh --configure-agents
```

Use `--version vX.Y.Z` to pin a release; use `--force` only to replace an
existing ARW installation. `arw update` is intentionally not available until a
release has been verified; rerun the versioned installer instead.

`-ConfigureAgents` on Windows and `--configure-agents` on Linux add the local,
typed `arw-mcp` service to Codex, Claude Code, and OpenCode. Restart those
applications after installing. Linux OpenCode setup uses Python 3's standard
JSON library to preserve existing user configuration.

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
installers/                      Versioned-release installers
cmd/arw/                         v2 Go CLI
cmd/arw-mcp/                     v2 stdio MCP server
internal/                        v2 Git, ledger, task, review, worktree logic
schemas/                         JSON contracts for tasks and review snapshots
integrations/                    Natural-language rules for each supported agent
```

## License

[MIT](LICENSE)
