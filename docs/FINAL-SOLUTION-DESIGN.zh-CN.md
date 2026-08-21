# Agent Review Workflow 最终方案设计

> 状态：已确认的目标架构，供后续实现窗口直接执行。  
> 版本：v2 设计稿。  
> 重要：本文是 v2 的目标架构，不是实现状态清单。以 README、源码和测试为准；ARW 不提供 VS Code 扩展，代码 diff 复用 VS Code Source Control 或 GitLens。

## 1. 目标与边界

Agent Review Workflow（ARW）让开发者以自然语言管理 AI 编写代码的任务，并在 IDE 中高效审查完整变更。

它要解决的是：AI 可能很快地改了许多代码，但人需要始终清楚知道“哪个任务改了什么、相对什么基准改、能否审、依赖谁、审查结论是否仍有效”。

### 1.1 必须达成的结果

- 以**任务**而不是日期或临时分支作为工作单元。
- 每个任务保存分支、基准提交、单一父任务、生命周期和人工审查结论。
- 支持独立任务和堆叠任务（例如 `main <- A <- B`）。
- 在 Codex、Claude Code、OpenCode 中用自然语言触发受限的工作流操作。
- 在 Windows 和 Linux 上使用同一个短命令 `arw`，且不依赖保留本地源码仓库。
- 输出精确的审查 base/head SHA，使开发者可在 VS Code Source Control 或 GitLens 中查看正确范围的 diff。
- 不操纵编辑器窗口；审查材料只返回 worktree 路径与精确的比较范围。
- 审查、合并、推送和代码改写都有可审计、可确认的边界。

### 1.2 明确不做的事

- 不让 AI 仅凭自身判断批准或合并任务，也不允许其自行推送、rebase、reset、强推、删除分支或删除 worktree。用户明确表达批准或本地合并意图后，Agent 可以调用对应的受限工具代为执行。
- 不提供 `run_arw("任意 shell 文本")` 这类泛化的 Agent 工具。
- 不用鼠标/屏幕自动化操纵 VS Code 内部 UI。
- 不构建或维护自定义 IDE diff 界面；ARW 复用开发者已有的 Git 工具。
- 不把 `.env`、令牌、数据库密码、完整源代码 diff 上传到中心服务；系统是本地优先的。

## 2. 总体架构

```text
用户的自然语言
        |
Codex / Claude Code / OpenCode
        |
统一规则与各自的 Skill/适配配置
        |
ARW MCP（仅暴露结构化、受限的任务工具）
        |
arw 核心程序（唯一的任务与 Git 业务实现）
        |--------------------|
        |                    |
Git / worktree         任务台账元数据分支
        |                    |
        +---- 审查快照（base/head SHA） ----> VS Code Source Control / GitLens
```

### 2.1 技术决策

| 层 | 技术 | 决策理由 |
| --- | --- | --- |
| 核心 CLI `arw` | Go | Windows/Linux 独立二进制、安装简单、适合可靠调度 Git 命令。 |
| MCP 服务 `arw-mcp` | Go | 与核心共享同一状态机和校验，避免再维护一套 Node 业务逻辑。 |
| 差异查看 | VS Code Source Control / GitLens | 复用成熟 Git UI；ARW 只提供任务上下文和精确 SHA。 |
| 安装器 | PowerShell + POSIX Shell | 仅下载、校验、配置和迁移；不承载业务逻辑。 |
| 层间契约 | JSON Schema | CLI、MCP 和测试共同使用的稳定数据模型。 |

语言不是安全边界。避免错误 Git 操作依靠严格的命令白名单、状态校验、明确确认与审计记录，而不是依靠某种语言本身。

## 3. 关键概念

| 名称 | 含义 |
| --- | --- |
| 任务（Task） | 一个可独立理解、测试和审查的工作单元。 |
| 基准（Base） | 创建或审查任务时使用的具体 Git 提交，而不仅是 `main` 这个会移动的名字。 |
| 父任务（Parent） | 当前任务直接建立在其上的任务，例如 B 建立在 A 上。 |
| 审查材料（Review Snapshot） | `prepare_review` 即时计算的 `base SHA`、`head SHA`、文件统计、依赖与批准有效性；不写入台账。 |
| 搁置（Parked） | 暂不推进但保留任务和历史；与放弃不同。 |
| Worktree | 同一 Git 仓库的独立工作目录，用来同时保留不同任务的代码现场。 |

## 4. 任务模型与状态机

不要把所有信息压进一个字符串状态，也不要仅根据分支是否存在来推断待审任务。任务有三个独立维度。

### 4.1 生命周期

```text
active -> ready_for_review -> merged
   |              |             |
   +------------> parked <------+
   |
   +------------> abandoned
```

| 值 | 含义 |
| --- | --- |
| `active` | 正在开发或等待继续开发。 |
| `ready_for_review` | 实现已提交审查；是否已批准由审查结论单独记录。 |
| `parked` | 暂不推进；可带有已审或未审结论。 |
| `merged` | 已确认进入目标分支。 |
| `abandoned` | 明确不再需要；保留历史和原因。 |

### 4.2 审查结论

| 值 | 含义 |
| --- | --- |
| `none` | 尚未有人记录审查结论。 |
| `changes_requested` | 审查要求修改。 |
| `approved` | 对本次基准和 HEAD 的人工审查通过。 |

批准是否仍有效不是持久化状态：`prepare_review` 根据当前 base、HEAD 和父任务即时派生为 `not_approved`、`current` 或 `stale`，避免出现互相矛盾的生命周期与审查状态。

### 4.3 依赖状态

依赖状态是派生值，不要求人工维护。

| 值 | 含义 |
| --- | --- |
| `clear` | 没有阻塞当前最终审查的依赖。 |
| `awaiting_prerequisite_review` | 父任务未最终审查/未合入。 |
| `parent_changed` | 父任务在当前任务审查后发生变化，必须复核。 |
| `blocked` | 父任务记录不可用等无法计算依赖的异常。 |

### 4.4 任务记录示例

任务记录必须是可读、可版本化且易于迁移的 YAML。每个任务一个文件，避免多人同时编辑一个总索引。

```yaml
schema_version: 1
id: fix-login-redirect
title: 修复登录跳转
branch: arw/fix-login-redirect
base:
  ref: main
  sha: 1a2b3c4d
parent_task: null
lifecycle: ready_for_review
review:
  status: none
  reviewed_base_sha: null
  reviewed_head_sha: null
created_at: 2026-08-12T09:00:00+08:00
updated_at: 2026-08-12T11:00:00+08:00
```

## 5. 台账存储与同步

### 5.1 采用专用元数据分支

任务记录保存在同一仓库的专用分支 `arw/registry` 中，例如：

```text
.agent-review/
  tasks/
    fix-login-redirect.yaml
    add-search.yaml
  schema-version.json
```

理由：

- 任务元数据不污染 `main` 或功能分支的产品代码提交。
- 元数据可随 Git 备份、同步到另一台机器并恢复。
- 每个任务独立文件，冲突范围较小。
- CLI 可通过读取该分支列出所有任务，而不需要从模糊的分支名猜测状态。

`arw` 内部维护该分支；用户无需日常 checkout 它。元数据的发布和同步必须是明确动作，不能被 Agent 静默推送。

### 5.2 本地状态

本机特有信息放在 Git 忽略的位置，例如 `.git/arw/`：

- worktree 的绝对路径；
- 当前工作区占用锁；
- 安装与诊断缓存；
- 不含代码、密钥或环境文件的临时审查产物。

台账永远不保存 `.env` 内容、令牌或完整源代码正文。

## 6. 分支与依赖策略

### 6.1 独立任务

```text
main
 ├─ arw/fix-login-redirect
 └─ arw/fix-database
```

两个任务均从创建时记录的 `main@SHA` 开始，彼此独立，可按任意顺序审查。

### 6.2 堆叠任务

当 B 必须使用 A 中尚未合入的接口或代码时：

```text
main <- arw/feature-a <- arw/feature-b
```

规则：

1. 审查 B 时，比较 `A...B`，只看 B 相对 A 的改动。
2. 如果 A 尚未验收，仍可查看 B 的审查材料，但不能记录 B 的最终批准。
3. 建议先审 A；用户仍可自由先审 B。
4. 用户明确批准 B 并单独要求合并后，可将 B 快进合入记录的父分支 A；此时 A 的 HEAD 已变化，A 的批准有效性变为 `stale`，需要重新审查后才能继续向上合并。
5. 工具不自动解决冲突，也不会把批准等同于合并或推送授权。

### 6.3 审查失效规则

以下任意情形发生，`prepare_review` 返回的批准有效性为 `stale`：

- 任务 HEAD 改变；
- 审查基准 SHA 改变；
- 子任务批准后父任务发生变化；
- 任务被 rebase 到不同基准；
- 合并冲突解决产生了新的代码。

系统可提示“快速复核”或“完整复核”，但不得把旧批准自动视为对新代码的批准。

## 7. Worktree 与并发规则

建议目录布局：

```text
C:\code\my-app
C:\code\my-app-worktrees\
  fix-login-redirect\
  fix-database\
```

规则：

- 一个任务对应一个 worktree；一个 worktree 同一时刻只允许一个写入 Agent。
- `arw task start` 在当前项目的配置位置创建任务 worktree。
- 当前 VS Code 窗口通常就是当前任务的 worktree；ARW 不会因审查其他任务而切换或打开窗口。
- 不自动 stash 未提交改动。开始新任务或切换上下文时，如工作区不干净，必须报告并要求用户决定。
- `.env`、数据库、端口、生成依赖不由 Git worktree 自动复制。任务可声明运行前置条件，避免不同 worktree 争用同一数据库或端口。

## 8. 用户自然语言与动作契约

自然语言由 Agent 解析，但每种意图只能映射到一个受限的结构化工具，而非任意 shell 命令。

| 用户表达 | 默认动作 | 不会发生的动作 |
| --- | --- | --- |
| “开始修复登录跳转” | 创建/定位任务、分支、worktree、基准记录。 | 不会改 `main`、不会推送。 |
| “有哪些待审任务？” | 列出 `ready_for_review`，并标出依赖阻塞。 | 不把所有分支混入结果。 |
| “审查登录跳转” | 即时生成审查材料和对话摘要。 | 不切换 VS Code、不合并。 |
| “在 VS Code 中审查数据库修复” | 生成审查材料，返回目标 worktree 与 SHA 范围。 | 不操纵编辑器窗口。 |
| “先搁置功能 A” | 记录为 `parked`，保留审查结论。 | 不删除分支或代码。 |
| “A 审查通过” | 复述任务、基准、HEAD 和影响；确认后记录人工批准。 | 不自动 merge/push。 |
| “把 A 合并到父分支” | 校验批准与版本，快进合入记录的父/base 分支并更新台账。 | 不选择任意目标、不解决冲突、不 push。 |

当用户要求审查 B，而 B 依赖尚未审查的 A 时，Agent 必须先提示：

```text
B 依赖 A；推荐先审 A。
如继续审 B，本次只比较 A...B；可以查看改动，但在 A 批准前不能记录 B 的最终批准。
```

这只是建议，不是强制。用户有权先审 B。

## 9. `arw` CLI 和机器接口

面向人类的命令保持简短；面向 MCP 的输出使用稳定 JSON。

```text
arw setup
arw doctor
arw task start "修复登录跳转"
arw task list [--view reviewable|active|parked|blocked]
arw task show <task-id>
arw task park <task-id>
arw task resume <task-id>
arw task merge --confirm <task-id>
arw task abandon --confirm <task-id>

arw review prepare <task-id>
arw review approve --confirm --base <reviewed-base-sha> --head <reviewed-head-sha> <task-id>
arw review request-changes <task-id> [--reason text]
```

所有可供程序消费的子命令支持：

```text
--format json
```

例如 `arw review prepare fix-login-redirect --format json` 至少返回：

```json
{
  "taskId": "fix-login-redirect",
  "worktree": "C:\\code\\my-app-worktrees\\fix-login-redirect",
  "base": { "ref": "main", "sha": "1a2b3c4d" },
  "head": { "sha": "7d8e9f00" },
  "comparison": "main@1a2b3c4d...HEAD@7d8e9f00",
  "files": [],
  "commits": [],
  "workingTree": "clean",
  "dependencyStatus": "clear",
  "reviewStatus": "none",
  "approvalValidity": "not_approved",
  "risks": []
}
```

## 10. MCP 与多 Agent 接入

`arw-mcp` 是本机 stdio MCP 服务。它包装核心库，不复制 Git 逻辑。

允许暴露的工具：

```text
workflow_list_tasks(filter)
workflow_get_task(task_id)
workflow_start_task(title, parent_task?)
workflow_prepare_review(task_id)
workflow_park_task(task_id)
workflow_resume_task(task_id)
workflow_mark_ready(task_id)
workflow_approve_task(task_id, expected_base_sha, expected_head_sha, confirm)
workflow_request_changes(task_id, reason?)
workflow_merge_task(task_id, confirm)
workflow_abandon_task(task_id, confirm)
```

不暴露的工具：

```text
任意 shell 执行
git push
任意目标或参数的 git merge
git rebase
git reset
git branch --delete
删除 worktree
```

人工审查结论和本地合并属于高影响动作。只有用户明确表达“通过”“要求修改”或“合并到父分支”等意图时，Agent 才能调用相应的受限操作。批准必须同时提交用户实际看过的 expected base/head；当前版本不一致时拒绝，不能用新快照替换。`confirm=true` 用于防误触，但不是身份认证；上层 Agent 规则仍必须保证意图来自用户。

`workflow_merge_task` 不接受任意目标分支：目标由 `parent_task` 或任务创建时的 base ref 推导。它要求任务处于当前批准状态、源和目标 worktree 干净、目标 HEAD 仍等于审查 base，并只执行 `--ff-only` 本地合并；任何冲突或目标移动都会停止，且永不 push。

各 Agent 的适配层负责：

- 安装统一的安全规则和自然语言映射；
- 配置本地 `arw-mcp`；
- 告知 Agent：已知任务先读取 `workflow_get_task`，未知任务用 `workflow_list_tasks`；
- 使 Codex、Claude Code、OpenCode 调用同一组工具和同一状态机。

## 11. 审查界面边界

ARW 不实现自定义 VS Code 扩展，也不试图替代 Git 的历史和 diff 界面。`arw review prepare <task-id>` 即时返回本次审查的精确 `base SHA` 和 `head SHA`，以及任务、依赖和风险上下文；该查询不写入台账。

- 常规任务审查：在对应 task worktree 中使用 VS Code Source Control / Source Control Graph，对比任务分支与记录的 base 分支或提交。
- 任意两个历史提交的全量比较：使用 GitLens 或 `git diff <base-sha> <head-sha>`。
- 审查结论：仍由用户通过对话明确表达，ARW/MCP 把批准绑定到实际查看的 base/head；展示工具不负责改变任务状态。

因此，ARW 的价值是“审查结论和合并约束”，不是“再造一个 diff 面板”。

## 12. 安全、确认与审计

### 12.1 安全级别

| 级别 | 示例 | 策略 |
| --- | --- | --- |
| 只读 | 列表、状态、diff、诊断 | 可直接执行。 |
| 本地可逆 | 创建任务分支/worktree、搁置、恢复、准备审查 | 必须来自用户的明确任务意图。 |
| 人工结论 | 通过、要求修改、放弃 | 必须有用户明确表达。 |
| 受限本地合并 | 将已批准任务快进到记录的父/base 分支 | 必须由用户单独明确要求；不接受任意目标、不解决冲突。 |
| 高风险/远程 | push、rebase、reset、删除、任意 merge | 不由 MCP 暴露；需要独立授权和外部工具。 |

### 12.2 审计内容

任务台账记录：

- 任务 ID、操作者标识、时间；
- base SHA、head SHA、父任务状态；
- 变更文件/提交统计；
- 人工审查结论及其绑定的 base/head SHA。

`prepare_review` 的文件统计、提交列表、脏工作区和派生有效性只在调用时计算并返回，不写入台账、也不为一次查看创建 Git 提交。

不记录源代码全文、密钥和环境变量。

## 13. 分发、安装和升级

用户不应依赖任意本地克隆目录。发布物包括：

```text
GitHub Release
├─ arw_windows_amd64.exe
├─ arw_linux_amd64
├─ arw_linux_arm64
├─ arw-mcp_...
└─ checksums.txt / provenance
```

安装脚本仅：

1. 检测平台与架构；
2. 下载指定版本的发布物；
3. 校验 SHA-256；
4. 放入用户数据目录并把 `bin` 加入 PATH；
5. 写入 Codex、Claude Code、OpenCode 的受控配置块；

运行时位置：

| 平台 | 位置 |
| --- | --- |
| Windows | `%LOCALAPPDATA%\\AgentReviewWorkflow` |
| Linux | `$XDG_DATA_HOME/agent-review-workflow`，默认 `~/.local/share/agent-review-workflow` |

升级必须显式重跑版本固定的安装器，并显示版本、校验结果和变更摘要。不得从 GitHub `main` 分支直接下载并执行未经版本固定的业务脚本。

## 14. 仓库结构

```text
agent-review-workflow/
├─ cmd/
│  ├─ arw/                     Go CLI 入口
│  └─ arw-mcp/                 Go MCP server 入口
├─ internal/
│  ├─ git/                     Git 命令封装与安全校验
│  ├─ task/                    状态机与任务模型
│  ├─ ledger/                  arw/registry 读写与同步
│  ├─ review/                  审查快照、比较范围、失效规则
│  └─ worktree/                worktree 创建、发现、锁定
├─ schemas/                    JSON Schema 与契约测试样本
├─ integrations/
│  ├─ codex/
│  ├─ claude-code/
│  └─ opencode/
├─ installers/                 PowerShell/Shell 安装、升级、迁移
├─ docs/
│  └─ FINAL-SOLUTION-DESIGN.zh-CN.md
└─ .github/workflows/
```

## 15. 实施分期与验收标准

### Phase 0：规范冻结与仓库准备

- 加入本文、JSON Schema 初稿、架构决策记录（ADR）。
- 标明 v2 的目标能力与明确边界。
- 验收：新开发者只读文档即可理解范围、不可自动化的操作及任务关系。

### Phase 1：Go Core MVP

- 实现 `doctor`、任务创建、任务列表、任务详情、worktree 创建。
- 建立 `arw/registry` 并写入/读取 YAML 任务记录。
- 验收：Windows/Linux 可从 Release 安装；创建独立任务后可恢复其信息。

### Phase 2：审查快照与依赖图

- 实现正确的 `base...head` 比较、未提交改动检测、提交/文件统计。
- 实现单父任务、批准门禁和批准有效性派生。
- 验收：`main <- A <- B` 下，B 不会被错误标记为最终通过。

### Phase 3：MCP 与 Agent 适配

- 实现受限 MCP 工具和统一 JSON Schema。
- 配置 Codex、Claude Code、OpenCode 的说明与 MCP 入口。
- 验收：三种 Agent 对同一任务得到一致的分支、状态和审查基准；无泛用 shell 工具。

### Phase 4：发布、升级与稳固

- Release 自动构建、SHA-256、构建来源证明、升级与迁移。
- Windows/Linux 集成测试、MCP 契约测试。
- 验收：在新机器中从公开地址安装后，不依赖原始克隆目录即可正常使用。

## 16. 测试策略

- **单元测试**：状态机转移、依赖派生、审查失效规则、任务 YAML 序列化。
- **Git 集成测试**：临时仓库中验证独立任务、堆叠任务、脏工作区、main 前进、冲突后的失效。
- **跨平台测试**：Windows 与 Linux 的安装、PATH、worktree、CLI JSON 输出。
- **MCP 契约测试**：每个工具的输入/输出均通过 JSON Schema，并验证禁止的动作不可调用。
- **人工验收**：用 Codex、Claude Code、OpenCode 各跑一遍“开始任务 → 写代码 → 审查 → 搁置 → 恢复 → 审查依赖任务”。

## 17. 最终验收场景

用户在任一支持的 AI 中说：

> 审查数据库修复。

系统必须能够：

1. 找到“数据库修复”任务，或在存在歧义时要求澄清；
2. 检查其分支、worktree、基准 SHA 和依赖；
3. 若依赖未验收，建议先审父任务，并明确此时不能记录最终批准；
4. 即时计算审查材料和风险摘要，不写入台账；
5. 返回目标 worktree 与精确的 base/head SHA，用户用 VS Code Source Control 或 GitLens 查看变更；
7. 在用户未单独要求时不 merge，并且不自行 push、rebase、reset 或删除任何工作内容。

满足这一场景，即说明 ARW 的核心价值已真正落地。
