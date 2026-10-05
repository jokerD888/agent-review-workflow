# Agent Review Workflow (ARW)

A portable, Git-first task workflow for AI coding agents (Antigravity, Codex, Claude Code, OpenCode).

ARW provides worktree isolation, immutable ledger branches, explicit human review boundaries, and validated fast-forward merges.

---

## 架构模型 (Architecture)

ARW 采用 **`Core + CLI + Wrapper Skill`** 三层架构，并将 **MCP** 明确定义为复用同一 Core 的可选适配器（Optional Adapter）：

```text
                               ┌── CLI ── Wrapper Skill (Default for Shell Hosts)
ARW Core (Unique Truth Source) ┤
                               └── MCP (Optional Adapter for Shell-less / SaaS)
```

1. **Skill（工作流大脑）**：负责场景识别、生命周期流转决策与 Progressive Disclosure 编排。
2. **Wrapper Script（跨平台引导层）**：`scripts/arw.ps1` 与 `scripts/arw.sh`，仅负责平台与架构推断、定位对应二进制并原样透传参数和退出码，不包含任何业务逻辑。
3. **CLI（稳定 Shell API）**：面向 Agent Shell 暴露收敛的高层命令，完全屏蔽底层的 git worktree、merge-base、rev-parse、快照比对与 fast-forward 校验细节，提供统一的 `--json` 机器可读模式。
4. **Core（唯一安全边界）**：`internal/app` 及底层 Git/Ledger/Review 实现，保证所有安全不变量。
5. **MCP（可选适配层）**：保留作为无本地 Shell 权限宿主环境的结构化工具接口，与 CLI 平等直接复用 Core `app.Service`。

---

## Skill 原生分发 (Skill Distribution)

ARW 默认通过 Agent Skill 分发，无需修改用户系统的全局 `PATH`，也不会向日常 Agent 的全局指令文件中粗暴注入长篇规则。

### 1. 作用域支持 (Scopes)

- **Global Skill**（推荐个人开发）：安装在用户主目录下的 Agent Skill 目录中（多项目共享、按需激活）。
- **Repo-scoped Skill**（推荐团队与 CI）：安装在项目仓库根目录下（如 `.agents/skills/agent-review-workflow`），与项目版本绑定。

### 2. 安装 Skill

使用仓库提供的轻量安装脚本：

#### Windows (PowerShell)
```powershell
# 安装为全局 Skill (默认支持 Antigravity, Codex, Claude, OpenCode)
.\installers\install-skill.ps1 -Scope global

# 安装为指定仓库的 Repo Skill
.\installers\install-skill.ps1 -Scope repo -TargetRepo path\to\your\project
```

#### macOS / Linux (POSIX Shell)
```sh
# 安装为全局 Skill
sh ./installers/install-skill.sh --scope global

# 安装为指定仓库的 Repo Skill
sh ./installers/install-skill.sh --scope repo --target-repo /path/to/your/project
```

---

## 快速上手 (Quick Start)

### 1. 编译 ARW CLI
环境要求：Git 与 Go 1.26+。在仓库根目录执行：
```powershell
go build -o ./bin/arw.exe ./cmd/arw
```

### 2. 通过 Wrapper 或 CLI 运行
在任何 Git 仓库中初始化 ARW 台账分支：
```powershell
.\skills\agent-review-workflow\scripts\arw.ps1 setup --json
```

创建第一个受隔离的任务：
```powershell
.\skills\agent-review-workflow\scripts\arw.ps1 task start --id fix-login "修复登录跳转" --json
```

查看任务列表：
```powershell
.\skills\agent-review-workflow\scripts\arw.ps1 task list --json
```

---

## 主生命周期 (Lifecycle)

```text
[Start Task] ──► [Worktree 开发与自测] ──► [Mark Ready & Prepare Review]
                                                        │
[Clear 清理] ◄── [FF Merge 快进合并] ◄── [用户人工审查批准] ◄┘
```

1. **`arw task start --id <id> "<title>" --json`**：创建独立任务分支 `arw/<id>` 与专属物理隔离 worktree；
2. **在 Worktree 中开发**：仅修改当前任务相关代码并提交独立 commit；
3. **`arw task ready <id> --json`**：标记任务就绪；
4. **`arw review prepare <id> --json`**：生成审查快照，包含 Base...HEAD SHA 范围、修改文件 diff、依赖与工作区状态；
5. **用户人工审查并批准**：用户明确批准后，调用：
   `arw review approve --confirm --base <reviewed-base-sha> --head <reviewed-head-sha> <id> --json`
   若代码在审查后发生变动，Core 将自动拒绝并报告失效；
6. **`arw task merge --confirm <id> --json`**：严格 fast-forward 快进合入基线，绝不自动 push；
7. **`arw task clear --confirm <id> --json`**：清理分支和物理工作区，永久保留不可变审计台账。

---

## 从旧版本迁移 (Migration from Legacy v2)

如果你之前使用旧版 `installers/install.ps1` 或 `install.sh` 安装过 ARW：

1. **清理全局 Agent 指令文件中的注入规则**：
   打开以下文件，删除 `<!-- agent-review-workflow:begin -->` 至 `<!-- agent-review-workflow:end -->` 之间的标记块：
   - Codex: `~/.codex/AGENTS.md` (或 `$CODEX_HOME/AGENTS.md`)
   - Claude Code: `~/.claude/CLAUDE.md`
   - OpenCode: `~/.config/opencode/AGENTS.md`
2. **清理 PATH（可选）**：
   无需保留 `LOCALAPPDATA/AgentReviewWorkflow/bin` 或 `~/.local/share/agent-review-workflow/bin` 在系统 `PATH` 中。
3. **安装新版 Skill**：
   运行 `./installers/install-skill.ps1 -Scope global`（或 `.sh`），Agent 将仅在实际触发工作流时按需激活 Skill。

---

## 目录结构 (Repository Layout)

```text
skills/agent-review-workflow/   # Skill 源码树 (不提交平台二进制)
├── SKILL.md                   # 精炼的高层编排规范 (Workflow Brain)
├── scripts/
│   ├── arw.ps1                # 跨平台引导层 (PowerShell)
│   └── arw.sh                 # 跨平台引导层 (POSIX Shell)
└── references/
    └── workflow-lifecycle.md  # 深入状态机、依赖与异常恢复参考

cmd/
├── arw/                       # 稳定 CLI (Shell API)
│   ├── main.go
│   └── main_test.go
└── arw-mcp/                   # 可选 MCP 适配器 (Agent Tool API)
    └── main.go

internal/                      # Core 业务逻辑与唯一安全边界
├── app/                       # 核心业务服务与集成测试
├── git/                       # Git 管道与安全检查
├── ledger/                    # arw/registry 孤儿分支持久化
├── mcp/                       # MCP JSON-RPC 协议实现
├── review/                    # 快照计算、diff 分析与风险检测
├── task/                      # 任务模型与格式校验
└── worktree/                  # Worktree 物理隔离管理

installers/                    # 安装与分发工具
├── install-skill.ps1          # 原生 Skill 安装器 (Windows)
├── install-skill.sh           # 原生 Skill 安装器 (Linux/macOS)
├── install.ps1                # [Deprecated] 旧版全局安装器
└── install.sh                 # [Deprecated] 旧版全局安装器
```

---

## License

[MIT](LICENSE)
