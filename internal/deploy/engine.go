package deploy

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sms/internal/model"
	"sms/internal/store"
)

type Change struct {
	Action string
	Source string
	Target string
}

type Plan struct {
	Project     model.Project
	Rule        model.DeployRule
	SourcePath  string
	ContentRoot string
	Changes     []Change
	TempDir     string
}

type ApplyResult struct {
	Changes     []Change
	BackupPaths []string
}

type snapshot struct {
	Target  string `json:"target"`
	Existed bool   `json:"existed"`
	Path    string `json:"-"`
}

type Engine struct {
	Root  string
	Store *store.Store
}

func New(root string, projectStore *store.Store) *Engine {
	return &Engine{Root: root, Store: projectStore}
}

func (e *Engine) Prepare(project model.Project, rule model.DeployRule) (*Plan, error) {
	if err := rule.Validate(); err != nil {
		return nil, err
	}
	projectDir := e.Store.ProjectDir(project.Code)
	deployRoot := filepath.Join(projectDir, "deploy-files")
	sourcePath := filepath.Join(projectDir, filepath.FromSlash(rule.Source))
	if err := ensureWithin(deployRoot, sourcePath); err != nil {
		return nil, fmt.Errorf("invalid source: %w", err)
	}
	info, err := os.Lstat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("source %s: %w", sourcePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("source symlinks are not supported")
	}

	plan := &Plan{Project: project, Rule: rule, SourcePath: sourcePath}
	switch rule.Type {
	case "file":
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("source is not a regular file")
		}
		targetName := rule.TargetName
		if targetName == "" {
			targetName = filepath.Base(sourcePath)
		}
		if filepath.Base(targetName) != targetName || targetName == "." || targetName == ".." {
			return nil, fmt.Errorf("targetName must be a file name, not a path")
		}
		plan.ContentRoot = sourcePath
		plan.Changes = []Change{makeChange(sourcePath, filepath.Join(rule.TargetDir, targetName))}
	case "directory":
		if !info.IsDir() {
			return nil, fmt.Errorf("source is not a directory")
		}
		plan.ContentRoot = sourcePath
		changes, err := entryChanges(sourcePath, rule.TargetDir)
		if err != nil {
			return nil, err
		}
		plan.Changes = changes
	case "archive":
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("archive source is not a regular file")
		}
		tempRoot := filepath.Join(e.Root, "tmp", "deploy")
		if err := os.MkdirAll(tempRoot, 0o755); err != nil {
			return nil, err
		}
		tempDir, err := os.MkdirTemp(tempRoot, project.Code+"-")
		if err != nil {
			return nil, err
		}
		plan.TempDir = tempDir
		extractRoot := filepath.Join(tempDir, "extracted")
		if err := os.MkdirAll(extractRoot, 0o755); err != nil {
			plan.Close()
			return nil, err
		}
		if err := extractArchive(sourcePath, extractRoot, rule.ArchiveFormat); err != nil {
			plan.Close()
			return nil, err
		}
		contentRoot, err := resolveContentRoot(extractRoot, rule)
		if err != nil {
			plan.Close()
			return nil, err
		}
		plan.ContentRoot = contentRoot
		changes, err := entryChanges(contentRoot, rule.TargetDir)
		if err != nil {
			plan.Close()
			return nil, err
		}
		plan.Changes = changes
	}
	return plan, nil
}

func (p *Plan) Close() {
	if p != nil && p.TempDir != "" {
		_ = os.RemoveAll(p.TempDir)
		p.TempDir = ""
	}
}

func (e *Engine) Apply(plan *Plan, backup bool) (ApplyResult, error) {
	if plan == nil || len(plan.Changes) == 0 {
		return ApplyResult{}, fmt.Errorf("deployment plan is empty")
	}
	tempRoot := filepath.Join(e.Root, "tmp", "deploy")
	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return ApplyResult{}, err
	}
	rollbackDir, err := os.MkdirTemp(tempRoot, "rollback-")
	if err != nil {
		return ApplyResult{}, err
	}
	defer os.RemoveAll(rollbackDir)

	snapshots := make([]snapshot, len(plan.Changes))
	for i, change := range plan.Changes {
		snapshots[i] = snapshot{Target: change.Target, Path: filepath.Join(rollbackDir, fmt.Sprintf("%04d", i))}
		if _, err := os.Lstat(change.Target); err == nil {
			snapshots[i].Existed = true
			if err := copyPath(change.Target, snapshots[i].Path); err != nil {
				return ApplyResult{}, fmt.Errorf("snapshot %s: %w", change.Target, err)
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return ApplyResult{}, err
		}
	}

	result := ApplyResult{Changes: append([]Change(nil), plan.Changes...)}
	if backup {
		paths, err := e.persistBackup(plan, snapshots, rollbackDir)
		if err != nil {
			return ApplyResult{}, fmt.Errorf("create backup: %w", err)
		}
		result.BackupPaths = paths
	}

	for i, change := range plan.Changes {
		if err := replacePath(change.Source, change.Target); err != nil {
			rollbackErr := restoreSnapshots(snapshots[:i+1])
			if rollbackErr != nil {
				return ApplyResult{}, fmt.Errorf("apply %s: %v; rollback: %w", change.Target, err, rollbackErr)
			}
			return ApplyResult{}, fmt.Errorf("apply %s: %w", change.Target, err)
		}
	}
	return result, nil
}

func (e *Engine) persistBackup(plan *Plan, snapshots []snapshot, rollbackDir string) ([]string, error) {
	timestamp := time.Now().Format("20060102-150405")
	if plan.Rule.Type == "file" && len(snapshots) == 1 && snapshots[0].Existed {
		backupPath := snapshots[0].Target + "-" + timestamp
		if err := copyPath(snapshots[0].Path, backupPath); err != nil {
			return nil, err
		}
		return []string{backupPath}, nil
	}
	backupDir := filepath.Join(e.Store.ProjectDir(plan.Project.Code), "backups", "dirs")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, err
	}
	base := plan.Rule.Name + "-" + timestamp
	archivePath := filepath.Join(backupDir, base+".tar.gz")
	manifestPath := filepath.Join(backupDir, base+".manifest.json")
	if err := createTarGz(archivePath, rollbackDir, snapshots); err != nil {
		return nil, err
	}
	manifest, err := json.MarshalIndent(struct {
		Rule    string      `json:"rule"`
		Created time.Time   `json:"createdAt"`
		Entries interface{} `json:"entries"`
	}{Rule: plan.Rule.Name, Created: time.Now(), Entries: snapshots}, "", "  ")
	if err != nil {
		return nil, err
	}
	manifest = append(manifest, '\n')
	if err := os.WriteFile(manifestPath, manifest, 0o640); err != nil {
		return nil, err
	}
	return []string{archivePath, manifestPath}, nil
}

func entryChanges(contentRoot, targetDir string) ([]Change, error) {
	if err := ensureNotNested(contentRoot, targetDir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(contentRoot)
	if err != nil {
		return nil, err
	}
	var changes []Change
	for _, entry := range entries {
		if isMetadataPath(entry.Name()) {
			continue
		}
		source := filepath.Join(contentRoot, entry.Name())
		if info, err := os.Lstat(source); err != nil {
			return nil, err
		} else if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("content symlink %s is not supported", source)
		}
		changes = append(changes, makeChange(source, filepath.Join(targetDir, entry.Name())))
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("deployment content is empty")
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Target < changes[j].Target })
	return changes, nil
}

func makeChange(source, target string) Change {
	action := "add"
	if _, err := os.Lstat(target); err == nil {
		action = "replace"
	}
	return Change{Action: action, Source: source, Target: target}
}

func resolveContentRoot(extractRoot string, rule model.DeployRule) (string, error) {
	if rule.ContentPath != "" {
		clean := filepath.Clean(filepath.FromSlash(rule.ContentPath))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("contentPath escapes the archive root")
		}
		root := filepath.Join(extractRoot, clean)
		if err := ensureWithin(extractRoot, root); err != nil {
			return "", err
		}
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			return "", fmt.Errorf("contentPath %q is not an extracted directory", rule.ContentPath)
		}
		return root, nil
	}
	entries, err := effectiveEntries(extractRoot)
	if err != nil {
		return "", err
	}
	strip := rule.StripTopLevel
	if strip == "" {
		strip = "auto"
	}
	hasSingleDir := len(entries) == 1 && entries[0].IsDir()
	switch strip {
	case "always":
		if !hasSingleDir {
			return "", fmt.Errorf("stripTopLevel=always requires exactly one top-level directory")
		}
		return filepath.Join(extractRoot, entries[0].Name()), nil
	case "auto":
		if hasSingleDir {
			return filepath.Join(extractRoot, entries[0].Name()), nil
		}
		return extractRoot, nil
	case "never":
		return extractRoot, nil
	default:
		return "", fmt.Errorf("invalid stripTopLevel %q", strip)
	}
}

func effectiveEntries(root string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	filtered := entries[:0]
	for _, entry := range entries {
		if !isMetadataPath(entry.Name()) {
			filtered = append(filtered, entry)
		}
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("archive has no deployable content")
	}
	return filtered, nil
}

func ensureWithin(root, candidate string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	candidateAbs, err := filepath.Abs(candidate)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("%s is outside %s", candidate, root)
	}
	return nil
}

func ensureNotNested(source, target string) error {
	sourceAbs, _ := filepath.Abs(source)
	targetAbs, _ := filepath.Abs(target)
	rel, err := filepath.Rel(sourceAbs, targetAbs)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
		return fmt.Errorf("targetDir cannot be inside deployment source")
	}
	return nil
}

func replacePath(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(filepath.Dir(target), ".sms-new-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	staged := filepath.Join(temp, filepath.Base(target))
	if err := copyPath(source, staged); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(staged, target)
}

func restoreSnapshots(snapshots []snapshot) error {
	var restoreErrors []string
	for i := len(snapshots) - 1; i >= 0; i-- {
		snapshot := snapshots[i]
		if err := os.RemoveAll(snapshot.Target); err != nil {
			restoreErrors = append(restoreErrors, err.Error())
			continue
		}
		if snapshot.Existed {
			if err := copyPath(snapshot.Path, snapshot.Target); err != nil {
				restoreErrors = append(restoreErrors, err.Error())
			}
		}
	}
	if len(restoreErrors) > 0 {
		return errors.New(strings.Join(restoreErrors, "; "))
	}
	return nil
}

func createTarGz(path, rollbackDir string, snapshots []snapshot) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()
	gz := gzip.NewWriter(file)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for i, snapshot := range snapshots {
		if !snapshot.Existed {
			continue
		}
		prefix := fmt.Sprintf("%04d-%s", i, filepath.Base(snapshot.Target))
		if err := addPathToTar(tw, snapshot.Path, prefix); err != nil {
			return err
		}
	}
	return nil
}

func addPathToTar(tw *tar.Writer, source, archiveName string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("cannot back up symlink %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		name := archiveName
		if rel != "." {
			name = filepath.Join(archiveName, rel)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(name)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(tw, file)
			file.Close()
			return copyErr
		}
		return nil
	})
}
