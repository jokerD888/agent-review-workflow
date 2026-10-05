# ARW Workflow Lifecycle & Invariant Reference

This reference documents the formal state machine, safety invariants, stacked task chains, and recovery procedures enforced by ARW Core.

---

## 1. 任务状态机 (Task State Machine)

每个 ARW 任务保存在孤儿分支 `arw/registry` 下的 `.agent-review/tasks/<id>.yaml` 中。任务生命周期遵循确定的状态转移模型：

```text
               ┌─────────── park ───────────┐
               │                            ▼
[Start] ──► Active ◄──────── resume ───── Parked
               │
           ready (mark task ready)
               ▼
        ReadyForReview ── approve (user confirms) ──► Approved
          ▲         │                                    │
          │         └── request-changes                  ▼
          │                    │                     [Merged]
          └────────────────────┘                         │
                                                         ▼
                                                      [Clear]
                                       (deletes branch & worktree,
                                        retains registry record)
```

### 状态说明：
- **`active`**：正在专属 worktree 中编码或修改。无法直接记录批准或执行合并。
- **`ready_for_review`**：Agent 自测完成并准备审查。唯有处于此状态且 worktree 干净、依赖畅通时才可接受审查与记录批准。
- **`parked`**：临时搁置。保留分支、worktree 与审查历史，无法进行合并或批准。使用 `arw task resume <task-id>` 恢复。
- **`merged`**：已成功快进合并至目标基线。只读，终态。
- **`abandoned`**：任务废弃。保留分支、worktree 与台账。使用 `arw task clear --confirm <task-id>` 回收资源。

---

## 2. 核心安全不变量 (Core Safety Invariants)

以下规则由 `internal/app`、`internal/review`、`internal/task` 强制保证，CLI 与 Wrapper 绝不重复实现：

### 2.1 审查快照不变性 (Snapshot Invariant)
- `arw review prepare` 会捕获精确的：
  - `Base.SHA`：基线提交哈希
  - `Head.SHA`：任务分支最新提交哈希
  - `WorkingTree`：worktree 是否干净（`clean` / `dirty` / `unknown`）
- `arw review approve` 强制要求传入 `--base <sha>` 与 `--head <sha>`。
- **防止偷换代码 (Stale Approval Prevention)**：若用户审查后任务分支又有新 commit，或基线分支被更新，`Approve` 将直接拒绝并报错：
  ```text
  reviewed version changed: user reviewed <base>...<head>, current range is <curr_base>...<curr_head>
  ```
- 此时必须重新调用 `arw review prepare`，向用户展示最新 diff，并等待用户就新版本再次表态。

### 2.2 工作区干净度检查 (Clean Worktree Invariant)
- 创建任务（`task start`）、审查批准（`review approve`）与快进合并（`task merge`）均强制要求关联的工作区处于 `clean` 状态。
- 若存在未提交的修改或未跟踪文件：
  - Agent 必须在对应 worktree 内提交或者由用户确认处理；
  - 严禁未经用户允许运行 `git reset --hard` 或 `git clean`。

### 2.3 严格快进合并 (Fast-Forward Only Merge)
- `arw task merge` 永远执行 `git merge --ff-only`。
- 如果目标分支在审查后前移，或存在不可快进的分叉，合并直接中断，拒绝产生未被人工审查过的非快进 merge commit。
- 绝不自动执行 `git push`。

---

## 3. 层叠任务与依赖链 (Stacked Tasks & Dependency Chains)

ARW 支持基于现有任务创建子任务（Stacked Tasks）：
```sh
arw task start --parent <parent-task-id> --id <child-id> "<child-title>" --json
```

### 3.1 审查前提阻塞 (Prerequisite Approval)
- 当父任务尚未获得人工批准时，子任务的 `dependency_status` 为 `awaiting_prerequisite_review`。
- 子任务可以执行 `review prepare` 供用户查看相对父任务的改动（`Parent...Child`）。
- **Core 阻止提前批准**：在父任务得到用户批准之前，子任务的 `arw review approve` 将直接被 Core 拒绝。
- 建议引导用户：“父任务 `<parent-id>` 尚未批准，请先审查并批准父任务”。

### 3.2 父任务变更重审 (Parent Changed Invalidation)
- 若父任务批准后又追加了新 commit，子任务的依赖状态自动变为 `parent_changed`。
- Core 将阻止子任务批准，要求先对父任务的新状态进行重审。

### 3.3 有效目标回溯推导 (Effective Target Resolution)
- 当父任务被合并并清理（`arw task clear`）后，其本地分支已不存在。
- 此时子任务的合并目标会自动沿祖先链向上回溯，寻找到第一个存活的祖先分支或原始基线分支（如 `main`），确保子任务依然能够安全无缝快进合入主干。

---

## 4. 故障与边缘情况恢复 (Recovery Procedures)

### 场景 A：Worktree 处于 Dirty 状态
1. 检查修改内容：进入对应 worktree 执行 `git status`；
2. 若改动属于当前任务且完整：使用 `git add` 和 `git commit` 创建语义清晰的局部提交；
3. 若存在无关或误操作修改：与用户沟通确认是否需要丢弃或暂存；
4. 重新运行 `arw review prepare <task-id> --json`。

### 场景 B：Stale Approval 拒绝合并
1. 现象：运行 `arw task merge` 时报错 `target branch moved from reviewed base` 或 `approval is no longer current`；
2. 原因：在审查批准后，基线分支合入了其他任务，或者任务工作区产生了新提交；
3. 恢复流程：
   - 切换到任务 worktree；
   - 若基线移动：根据项目策略执行 `git rebase <base-branch>`；
   - 重新运行 `arw review prepare <task-id> --json` 获取新快照；
   - 提示用户查看更新后的变更范围；
   - 获得用户再次确认后，调用 `arw review approve` 并执行合并。

### 场景 C：父任务废弃 (Parent Abandoned)
- 若父任务被标记为 `abandoned`，依赖于它的子任务将进入 `blocked` 状态；
- 子任务无法自动合并到已废弃的分支；需由人类用户决定是否将子任务 rebase 到主干或单独处理。
