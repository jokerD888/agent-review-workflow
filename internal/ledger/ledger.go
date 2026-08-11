package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gitclient "github.com/jokerD888/agent-review-workflow/internal/git"
	"github.com/jokerD888/agent-review-workflow/internal/task"
	"gopkg.in/yaml.v3"
)

const RegistryBranch = "arw/registry"
const taskDirectory = ".agent-review/tasks"

type Store struct{ Git gitclient.Client }

func (s Store) Setup() error {
	if s.Git.BranchExists(RegistryBranch) {
		return nil
	}
	tree, err := s.Git.Run("mktree")
	if err != nil {
		return fmt.Errorf("create empty registry tree: %w", err)
	}
	commit, err := s.Git.Run("commit-tree", strings.TrimSpace(tree), "-m", "chore(arw): initialize task registry")
	if err != nil {
		return fmt.Errorf("initialize registry needs Git user.name and user.email: %w", err)
	}
	_, err = s.Git.Run("update-ref", "refs/heads/"+RegistryBranch, strings.TrimSpace(commit))
	if err != nil {
		return fmt.Errorf("create registry branch: %w", err)
	}
	return nil
}

func (s Store) List() ([]task.Task, error) {
	if !s.Git.BranchExists(RegistryBranch) {
		return []task.Task{}, nil
	}
	output, err := s.Git.Run("ls-tree", "-r", "--name-only", RegistryBranch, "--", taskDirectory)
	if err != nil {
		return nil, err
	}
	var tasks []task.Task
	for _, name := range strings.Fields(output) {
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		contents, err := s.Git.Run("show", RegistryBranch+":"+name)
		if err != nil {
			return nil, err
		}
		var entry task.Task
		if err := yaml.Unmarshal([]byte(contents), &entry); err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		if err := entry.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", name, err)
		}
		tasks = append(tasks, entry)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].CreatedAt < tasks[j].CreatedAt })
	return tasks, nil
}

func (s Store) Get(id string) (task.Task, error) {
	if !task.ValidID(id) {
		return task.Task{}, fmt.Errorf("invalid task id %q", id)
	}
	contents, err := s.Git.Run("show", RegistryBranch+":"+taskPath(id))
	if err != nil {
		return task.Task{}, fmt.Errorf("task %q not found in %s", id, RegistryBranch)
	}
	var entry task.Task
	if err := yaml.Unmarshal([]byte(contents), &entry); err != nil {
		return task.Task{}, fmt.Errorf("parse task %q: %w", id, err)
	}
	if err := entry.Validate(); err != nil {
		return task.Task{}, err
	}
	return entry, nil
}

func (s Store) Save(entry task.Task, message string) error {
	if err := entry.Validate(); err != nil {
		return err
	}
	if err := s.Setup(); err != nil {
		return err
	}
	data, err := yaml.Marshal(entry)
	if err != nil {
		return err
	}
	return s.commitFile(taskPath(entry.ID), string(data), message)
}

func (s Store) SaveSnapshot(id string, snapshot any) error {
	if err := s.Setup(); err != nil {
		return err
	}
	data, err := yaml.Marshal(snapshot)
	if err != nil {
		return err
	}
	name := time.Now().UTC().Format("20060102T150405.000000000Z") + ".yaml"
	return s.commitFile(filepath.ToSlash(filepath.Join(".agent-review", "reviews", id, name)), string(data), "chore(arw): snapshot review "+id)
}

func (s Store) commitFile(path, contents, message string) error {
	common, err := s.Git.CommonDir()
	if err != nil {
		return err
	}
	lockPath := filepath.Join(common, "arw", "registry.lock")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return err
	}
	if err := os.Mkdir(lockPath, 0o755); err != nil {
		return fmt.Errorf("registry is busy; try again shortly")
	}
	defer os.Remove(lockPath)
	index, err := os.CreateTemp(filepath.Join(common, "arw"), "registry-index-")
	if err != nil {
		return err
	}
	indexPath := index.Name()
	if err := index.Close(); err != nil {
		return err
	}
	defer os.Remove(indexPath)
	environment := []string{"GIT_INDEX_FILE=" + indexPath}
	previous, err := s.Git.Resolve(RegistryBranch)
	if err != nil {
		return err
	}
	if _, err := s.Git.RunWithInput("", environment, "read-tree", RegistryBranch); err != nil {
		return err
	}
	blob, err := s.Git.RunWithInput(contents, environment, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	if _, err := s.Git.RunWithInput("", environment, "update-index", "--add", "--cacheinfo", "100644,"+strings.TrimSpace(blob)+","+path); err != nil {
		return err
	}
	tree, err := s.Git.RunWithInput("", environment, "write-tree")
	if err != nil {
		return err
	}
	commit, err := s.Git.RunWithInput("", environment, "-c", "commit.gpgSign=false", "commit-tree", strings.TrimSpace(tree), "-p", previous, "-m", message)
	if err != nil {
		return fmt.Errorf("commit registry entry: %w", err)
	}
	if _, err := s.Git.Run("update-ref", "refs/heads/"+RegistryBranch, strings.TrimSpace(commit), previous); err != nil {
		return fmt.Errorf("update registry branch: %w", err)
	}
	return nil
}

func taskPath(id string) string { return taskDirectory + "/" + id + ".yaml" }
