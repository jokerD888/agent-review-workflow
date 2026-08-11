import * as childProcess from "node:child_process";
import * as util from "node:util";
import * as vscode from "vscode";

const execFile = util.promisify(childProcess.execFile);

interface Task { id: string; title: string; branch: string; base: { ref: string; sha: string }; parentTask?: string; lifecycle: string; review: { status: string } }
interface ReviewFile { path: string; status: string; additions: number; deletions: number }
interface Snapshot { taskId: string; base: { ref?: string; sha: string }; head: { sha: string }; comparison: string; files: ReviewFile[]; dependencyStatus: string; reviewStatus: string; risks: string[] }
type Category = "Reviewable" | "Active" | "Blocked" | "Parked" | "Completed";

class ArwClient {
  async run<T>(args: string[]): Promise<T> {
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) throw new Error("Open a local Git repository folder first.");
    const arwPath = vscode.workspace.getConfiguration("agentReviewWorkflow").get<string>("arwPath", "arw");
    try {
      const { stdout } = await execFile(arwPath, [...args, "--format", "json"], { cwd: folder.uri.fsPath, windowsHide: true, maxBuffer: 4 * 1024 * 1024 });
      return JSON.parse(stdout) as T;
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error);
      throw new Error(`ARW command failed: ${detail}`);
    }
  }
  async git(args: string[]): Promise<string> {
    const folder = vscode.workspace.workspaceFolders?.[0];
    if (!folder) throw new Error("Open a local Git repository folder first.");
    const { stdout } = await execFile("git", args, { cwd: folder.uri.fsPath, windowsHide: true });
    return stdout.trim();
  }
}

class ReviewContentProvider implements vscode.TextDocumentContentProvider {
  private readonly changed = new vscode.EventEmitter<vscode.Uri>();
  readonly onDidChange = this.changed.event;
  constructor(private readonly client: ArwClient) {}
  async provideTextDocumentContent(uri: vscode.Uri): Promise<string> {
    const sha = uri.authority;
    const filePath = decodeURIComponent(uri.path.replace(/^\//, ""));
    try { return await this.client.git(["show", `${sha}:${filePath}`]); }
    catch { return ""; } // Git cannot show the missing side of an added/deleted file.
  }
}

class CategoryItem extends vscode.TreeItem {
  constructor(readonly category: Category) { super(category, vscode.TreeItemCollapsibleState.Expanded); this.contextValue = "arwCategory"; this.iconPath = new vscode.ThemeIcon("list-tree"); }
}
class TaskItem extends vscode.TreeItem {
  constructor(readonly task: Task, readonly snapshot?: Snapshot) {
    super(task.title, snapshot ? vscode.TreeItemCollapsibleState.Expanded : vscode.TreeItemCollapsibleState.None);
    this.description = `${task.lifecycle} · ${snapshot?.reviewStatus ?? task.review.status}`;
    this.tooltip = `${task.id}\n${task.branch}\nBase: ${task.base.ref}@${task.base.sha}\nReview: ${this.description}`;
    this.contextValue = "arwTask";
    this.iconPath = new vscode.ThemeIcon(task.review.status === "approved" ? "pass" : "git-pull-request");
    this.command = { command: "agentReviewWorkflow.openTaskReview", title: "Open Task Review", arguments: [this] };
  }
}
class FileItem extends vscode.TreeItem {
  constructor(readonly task: Task, readonly snapshot: Snapshot, readonly file: ReviewFile) {
    super(file.path, vscode.TreeItemCollapsibleState.None);
    this.description = `${file.status} +${file.additions} −${file.deletions}`;
    this.tooltip = `${snapshot.comparison}\n${file.path}`;
    this.contextValue = "arwFile";
    this.iconPath = new vscode.ThemeIcon("diff");
    this.command = { command: "agentReviewWorkflow.openFileDiff", title: "Open File Diff", arguments: [this] };
  }
}

class TasksProvider implements vscode.TreeDataProvider<CategoryItem | TaskItem | FileItem> {
  private readonly changed = new vscode.EventEmitter<void>();
  readonly onDidChangeTreeData = this.changed.event;
  private tasks: Task[] = [];
  private snapshots = new Map<string, Snapshot>();
  constructor(private readonly client: ArwClient) {}
  refresh(): void { void this.load(); }
  async load(): Promise<void> { try { this.tasks = await this.client.run<Task[]>(["task", "list"]); this.changed.fire(); } catch (error) { vscode.window.showWarningMessage(error instanceof Error ? error.message : String(error)); } }
  setSnapshot(snapshot: Snapshot): void { this.snapshots.set(snapshot.taskId, snapshot); this.changed.fire(); }
  getTreeItem(element: CategoryItem | TaskItem | FileItem): vscode.TreeItem { return element; }
  getChildren(element?: CategoryItem | TaskItem | FileItem): vscode.ProviderResult<(CategoryItem | TaskItem | FileItem)[]> {
    if (!element) return ["Reviewable", "Active", "Blocked", "Parked", "Completed"].map(category => new CategoryItem(category as Category));
    if (element instanceof CategoryItem) return this.tasks.filter(task => inCategory(task, element.category)).map(task => new TaskItem(task, this.snapshots.get(task.id)));
    if (element instanceof TaskItem && element.snapshot) return element.snapshot.files.map(file => new FileItem(element.task, element.snapshot!, file));
    return [];
  }
  taskByBranch(branch: string): Task | undefined { return this.tasks.find(task => task.branch === branch); }
}

function inCategory(task: Task, category: Category): boolean {
  if (category === "Parked") return task.lifecycle === "parked";
  if (category === "Completed") return task.lifecycle === "merged" || task.lifecycle === "approved" || task.lifecycle === "abandoned";
  if (category === "Blocked") return Boolean(task.parentTask) && task.review.status !== "approved";
  if (category === "Reviewable") return task.lifecycle === "ready_for_review" || task.lifecycle === "in_review";
  return task.lifecycle === "active";
}

export function activate(context: vscode.ExtensionContext): void {
  const client = new ArwClient();
  const provider = new TasksProvider(client);
  const contentProvider = new ReviewContentProvider(client);
  context.subscriptions.push(vscode.window.registerTreeDataProvider("agentReviewWorkflow.tasks", provider));
  context.subscriptions.push(vscode.workspace.registerTextDocumentContentProvider("arw-review", contentProvider));
  const prepare = async (item: TaskItem): Promise<Snapshot | undefined> => {
    try { const snapshot = await client.run<Snapshot>(["review", "prepare", item.task.id]); provider.setSnapshot(snapshot); await vscode.commands.executeCommand("setContext", "agentReviewWorkflow.activeTask", item.task.id); vscode.window.showInformationMessage(`${item.task.title}: ${snapshot.comparison} (${snapshot.dependencyStatus})`); return snapshot; }
    catch (error) { vscode.window.showErrorMessage(error instanceof Error ? error.message : String(error)); return undefined; }
  };
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.refresh", () => provider.refresh()));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.openTaskReview", async (item: TaskItem) => { await prepare(item); }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.openFileDiff", async (item: FileItem) => {
    const path = encodeURIComponent(item.file.path).replace(/%2F/g, "/");
    const left = vscode.Uri.parse(`arw-review://${item.snapshot.base.sha}/${path}`);
    const right = vscode.Uri.parse(`arw-review://${item.snapshot.head.sha}/${path}`);
    await vscode.commands.executeCommand("vscode.diff", left, right, `${item.file.path} (${item.snapshot.base.ref ?? "base"} ↔ task)`);
  }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.reviewCurrentTask", async () => {
    try { const branch = await client.git(["branch", "--show-current"]); const item = provider.taskByBranch(branch); if (!item) { vscode.window.showWarningMessage(`The current branch (${branch || "detached HEAD"}) is not an ARW task.`); return; }; await prepare(new TaskItem(item)); } catch (error) { vscode.window.showErrorMessage(error instanceof Error ? error.message : String(error)); }
  }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.openTaskWorktree", async (item: TaskItem) => { try { await client.run(["worktree", "open", item.task.id]); } catch (error) { vscode.window.showErrorMessage(error instanceof Error ? error.message : String(error)); } }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.parkTask", async (item: TaskItem) => { const choice = await vscode.window.showWarningMessage(`Park ${item.task.title}? Its branch, worktree, and review history are retained.`, { modal: true }, "Park Task"); if (choice === "Park Task") { await client.run(["task", "park", item.task.id]); provider.refresh(); } }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.approveTask", async (item: TaskItem) => { const choice = await vscode.window.showWarningMessage(`Record human approval for ${item.task.title}? This does not merge or push.`, { modal: true }, "Record Approval"); if (choice === "Record Approval") { await client.run(["review", "approve", "--confirm", item.task.id]); provider.refresh(); } }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.requestChanges", async (item: TaskItem) => { const reason = await vscode.window.showInputBox({ prompt: `Reason for changes requested on ${item.task.title}` }); if (reason !== undefined) { await client.run(["review", "request-changes", "--reason", reason, item.task.id]); provider.refresh(); } }));
  context.subscriptions.push(vscode.commands.registerCommand("agentReviewWorkflow.showDependencyGraph", async () => { const tasks = await client.run<Task[]>(["task", "list"]); const panel = vscode.window.createWebviewPanel("arwDependencyGraph", "ARW Task Dependencies", vscode.ViewColumn.Beside, {}); panel.webview.html = dependencyHtml(tasks); }));
  provider.refresh();
}

function dependencyHtml(tasks: Task[]): string { const rows = tasks.map(task => `<li><strong>${escapeHtml(task.title)}</strong> <code>${escapeHtml(task.id)}</code>${task.parentTask ? ` ← depends on <code>${escapeHtml(task.parentTask)}</code>` : ""}</li>`).join(""); return `<!doctype html><html><body><h2>Task dependencies</h2><p>Arrows point from a task to its parent prerequisite.</p><ul>${rows}</ul></body></html>`; }
function escapeHtml(value: string): string { return value.replace(/[&<>'"]/g, character => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", "\"": "&quot;" }[character] ?? character)); }
export function deactivate(): void {}
