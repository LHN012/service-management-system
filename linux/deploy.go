package main

import (
	// archive/tar 用来解 tar/tar.gz/tgz。
	"archive/tar"
	// archive/zip 用来解 zip。
	"archive/zip"
	"bufio"
	// compress/gzip 用来读取 tar.gz/tgz。
	"compress/gzip"
	"errors"
	// fmt 用来输出部署结果和错误。
	"fmt"
	// io 用来复制文件内容。
	"io"
	// os 用来读写文件、创建目录、删除旧目标。
	"os"
	// filepath 用来安全拼接 deploy-files 和目标路径。
	"path/filepath"
	// strings 用来校验路径和显示文件类型。
	"strings"
	// time 用来生成备份目录名。
	"time"
)

type deploymentPlan struct {
	ProjectDir   string
	Source       string
	SourceType   string
	Target       string
	TargetExists bool
	TargetType   string
}

// isDeployCommand 判断当前输入是不是快速迁移命令。
// 支持：
//
//	dp <project>
//	dp <project> <source> <target>
//	deploy <project> <source> <target>
func isDeployCommand(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	return fields[0] == "dp" || fields[0] == "deploy"
}

// runDeploy lists deploy files, previews a plan, or applies a confirmed plan.
func runDeploy(scanner *bufio.Scanner, output io.Writer, root string, request commandRequest) error {
	projectName := request.Project
	if err := validateProjectName(projectName); err != nil {
		return err
	}

	project, _, err := loadProject(root, projectName)
	if err != nil {
		return err
	}

	projectDir := filepath.Join(root, "projects", project.Name)
	if err := ensureProjectDirs(projectDir); err != nil {
		return err
	}

	deployDir := filepath.Join(projectDir, "deploy-files")
	if request.Action == "deploy.list" {
		return printDeployFiles(output, deployDir)
	}

	plan, err := buildDeploymentPlan(projectDir, deployDir, request.Source, request.Target)
	if err != nil {
		return err
	}
	printDeploymentPlan(output, plan)
	if request.Action == "deploy.plan" {
		return nil
	}
	if request.Action != "deploy.apply" {
		return fmt.Errorf("unsupported deploy action: %s", request.Action)
	}
	if !request.Yes {
		confirmed, err := confirm(scanner, output, "apply this deployment plan")
		if err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(output, "deployment cancelled")
			return nil
		}
	}

	backupPath, backedUp, err := backupTarget(projectDir, plan.Target)
	if err != nil {
		return err
	}

	if err := replaceTarget(plan.Source, plan.Target); err != nil {
		return err
	}

	fmt.Fprintf(output, "deployed %s -> %s\n", plan.Source, plan.Target)
	if backedUp {
		fmt.Fprintf(output, "backup: %s\n", backupPath)
	} else {
		fmt.Fprintln(output, "backup: skipped, target did not exist")
	}
	return nil
}

func buildDeploymentPlan(projectDir string, deployDir string, sourceValue string, targetValue string) (deploymentPlan, error) {
	source, err := resolveDeploySource(deployDir, sourceValue)
	if err != nil {
		return deploymentPlan{}, err
	}
	target, err := resolveDeployTarget(source, targetValue)
	if err != nil {
		return deploymentPlan{}, err
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return deploymentPlan{}, err
	}
	sourceType := "file"
	if sourceInfo.IsDir() {
		sourceType = "directory"
	} else if isArchiveSource(source) {
		sourceType = "archive"
	}

	plan := deploymentPlan{ProjectDir: projectDir, Source: source, SourceType: sourceType, Target: target}
	if targetInfo, err := os.Lstat(target); err == nil {
		plan.TargetExists = true
		plan.TargetType = "file"
		if targetInfo.IsDir() {
			plan.TargetType = "directory"
		} else if targetInfo.Mode()&os.ModeSymlink != 0 {
			plan.TargetType = "symlink"
		}
	} else if !os.IsNotExist(err) {
		return deploymentPlan{}, err
	}
	return plan, nil
}

func printDeploymentPlan(output io.Writer, plan deploymentPlan) {
	fmt.Fprintln(output, "Deployment plan:")
	fmt.Fprintf(output, "  source: %s (%s)\n", plan.Source, plan.SourceType)
	fmt.Fprintf(output, "  target: %s\n", plan.Target)
	if plan.TargetExists {
		fmt.Fprintf(output, "  existing target: %s; backup required\n", plan.TargetType)
	} else {
		fmt.Fprintln(output, "  existing target: none")
	}
	fmt.Fprintln(output, "  strategy: stage, verify copy, then rename into place")
}

// printDeployFiles 展示项目 deploy-files 投放目录。
func printDeployFiles(output io.Writer, deployDir string) error {
	entries, err := os.ReadDir(deployDir)
	if err != nil {
		return err
	}

	fmt.Fprintf(output, "deploy-files: %s\n", deployDir)
	if len(entries) == 0 {
		fmt.Fprintln(output, "empty")
		return nil
	}

	fmt.Fprintf(output, "%-8s %s\n", "type", "name")
	for _, entry := range entries {
		kind := "file"
		if entry.IsDir() {
			kind = "dir"
		}
		fmt.Fprintf(output, "%-8s %s\n", kind, entry.Name())
	}
	return nil
}

// resolveDeploySource 把用户输入的源路径限制在 deploy-files 下。
func resolveDeploySource(deployDir string, source string) (string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", fmt.Errorf("source is required")
	}
	if filepath.IsAbs(source) {
		return "", fmt.Errorf("source must be relative to deploy-files")
	}
	deployInfo, err := os.Lstat(deployDir)
	if err != nil {
		return "", err
	}
	if deployInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("deploy-files directory cannot be a symbolic link")
	}

	clean := filepath.Clean(source)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		return "", fmt.Errorf("source cannot escape deploy-files")
	}

	path := filepath.Join(deployDir, clean)
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("deploy source cannot be a symbolic link: %s", source)
	}
	realDeployDir, err := filepath.EvalSymlinks(deployDir)
	if err != nil {
		return "", err
	}
	realSource, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if !pathWithin(realDeployDir, realSource) {
		return "", fmt.Errorf("source cannot escape deploy-files through a symbolic link")
	}
	if info.IsDir() {
		if err := rejectSymlinks(path); err != nil {
			return "", err
		}
	}
	return path, nil
}

// resolveDeployTarget 计算最终覆盖目标。
// 文件源如果指向现有目录或以分隔符结尾，则复制到该目录下同名文件。
func resolveDeployTarget(source string, target string) (string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return "", fmt.Errorf("target is required")
	}
	if !filepath.IsAbs(target) {
		return "", fmt.Errorf("target must be an absolute path")
	}

	cleanTarget := filepath.Clean(target)
	if isRootPath(cleanTarget) {
		return "", fmt.Errorf("target cannot be filesystem root")
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}

	if !sourceInfo.IsDir() && isArchiveSource(source) {
		if targetInfo, err := os.Stat(cleanTarget); err == nil && !targetInfo.IsDir() {
			return "", fmt.Errorf("archive source requires a target directory")
		}
		return cleanTarget, nil
	}

	if !sourceInfo.IsDir() {
		if targetEndsWithSeparator(target) {
			return filepath.Join(cleanTarget, filepath.Base(source)), nil
		}
		if targetInfo, err := os.Stat(cleanTarget); err == nil && targetInfo.IsDir() {
			return filepath.Join(cleanTarget, filepath.Base(source)), nil
		}
		return cleanTarget, nil
	}

	if targetInfo, err := os.Stat(cleanTarget); err == nil && !targetInfo.IsDir() {
		return "", fmt.Errorf("source is a directory but target is a file")
	}
	return cleanTarget, nil
}

// backupTarget 覆盖前备份目标。
func backupTarget(projectDir string, target string) (string, bool, error) {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}

	backupsDir := filepath.Join(projectDir, "backups")
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return "", false, err
	}
	backupRoot, err := os.MkdirTemp(backupsDir, time.Now().Format("20060102-150405")+"-")
	if err != nil {
		return "", false, err
	}
	backupPath := filepath.Join(backupRoot, backupRelativePath(target))
	if err := copyPath(target, backupPath); err != nil {
		_ = os.RemoveAll(backupRoot)
		return "", false, err
	}
	return backupPath, true, nil
}

// replaceTarget builds the complete replacement beside the target before
// renaming it into place. If the final rename fails, the previous target is
// restored immediately.
func replaceTarget(source string, target string) error {
	staged, err := buildStagedTarget(source, target)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staged)
		}
	}()

	if err := commitStagedTarget(staged, target); err != nil {
		return err
	}
	committed = true
	return nil
}

func buildStagedTarget(source string, target string) (string, error) {
	parent := filepath.Dir(target)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return "", err
	}

	if !sourceInfo.IsDir() && !isArchiveSource(source) {
		file, err := os.CreateTemp(parent, ".sms-stage-"+filepath.Base(target)+"-*")
		if err != nil {
			return "", err
		}
		staged := file.Name()
		if err := file.Close(); err != nil {
			_ = os.Remove(staged)
			return "", err
		}
		if err := copyFile(source, staged); err != nil {
			_ = os.Remove(staged)
			return "", err
		}
		return staged, nil
	}

	staged, err := os.MkdirTemp(parent, ".sms-stage-"+filepath.Base(target)+"-*")
	if err != nil {
		return "", err
	}
	stagedMode := os.FileMode(0o755)
	if sourceInfo.IsDir() && sourceInfo.Mode().Perm() != 0 {
		stagedMode = sourceInfo.Mode().Perm()
	}
	if err := os.Chmod(staged, stagedMode); err != nil {
		_ = os.RemoveAll(staged)
		return "", err
	}
	success := false
	defer func() {
		if !success {
			_ = os.RemoveAll(staged)
		}
	}()

	payload := source
	if !sourceInfo.IsDir() {
		extractRoot, err := os.MkdirTemp("", "sms-deploy-extract-*")
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(extractRoot)
		if err := extractArchive(source, extractRoot); err != nil {
			return "", err
		}
		payload, err = archivePayloadDir(extractRoot)
		if err != nil {
			return "", err
		}
	}
	if err := copyDirContents(payload, staged); err != nil {
		return "", err
	}
	success = true
	return staged, nil
}

func commitStagedTarget(staged string, target string) error {
	parent := filepath.Dir(target)
	previous := ""
	if _, err := os.Lstat(target); err == nil {
		placeholder, err := os.CreateTemp(parent, ".sms-previous-"+filepath.Base(target)+"-*")
		if err != nil {
			return err
		}
		previous = placeholder.Name()
		if err := placeholder.Close(); err != nil {
			return err
		}
		if err := os.Remove(previous); err != nil {
			return err
		}
		if err := os.Rename(target, previous); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.Rename(staged, target); err != nil {
		if previous != "" {
			rollbackErr := os.Rename(previous, target)
			return errors.Join(fmt.Errorf("activate staged target: %w", err), rollbackErr)
		}
		return fmt.Errorf("activate staged target: %w", err)
	}
	if previous != "" {
		if err := os.RemoveAll(previous); err != nil {
			return fmt.Errorf("deployment completed but previous target cleanup failed: %w", err)
		}
	}
	return nil
}

// extractArchive 按后缀解压压缩包。
func extractArchive(source string, target string) error {
	lower := strings.ToLower(source)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(source, target)
	case strings.HasSuffix(lower, ".tar"):
		return extractTarFile(source, target)
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(source, target)
	default:
		return fmt.Errorf("unsupported archive format: %s", source)
	}
}

// extractZip 解压 zip 文件。
func extractZip(source string, target string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		destination, err := safeExtractPath(target, file.Name)
		if err != nil {
			return err
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive symbolic links are not supported: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}

		sourceFile, err := file.Open()
		if err != nil {
			return err
		}
		if err := copyReaderToFile(sourceFile, destination, file.FileInfo().Mode().Perm()); err != nil {
			sourceFile.Close()
			return err
		}
		sourceFile.Close()
	}
	return nil
}

// extractTarFile 解压 tar 文件。
func extractTarFile(source string, target string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	return extractTar(tar.NewReader(file), target)
}

// extractTarGz 解压 tar.gz/tgz 文件。
func extractTarGz(source string, target string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()

	return extractTar(tar.NewReader(gzipReader), target)
}

// extractTar 解压 tar 流。
func extractTar(reader *tar.Reader, target string) error {
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		destination, err := safeExtractPath(target, header.Name)
		if err != nil {
			return err
		}

		mode := os.FileMode(header.Mode).Perm()
		switch header.Typeflag {
		case tar.TypeDir:
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(destination, mode); err != nil {
				return err
			}
			if err := os.Chmod(destination, mode); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := copyReaderToFile(reader, destination, mode); err != nil {
				return err
			}
		case tar.TypeSymlink, tar.TypeLink:
			return fmt.Errorf("archive links are not supported: %s", header.Name)
		default:
			continue
		}
	}
}

// safeExtractPath 防止压缩包里带 ../ 逃逸目标目录。
func safeExtractPath(root string, name string) (string, error) {
	name = filepath.Clean(strings.TrimLeft(name, "/\\"))
	if name == "." || name == "" {
		return root, nil
	}
	if name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes target: %s", name)
	}

	destination := filepath.Join(root, name)
	relative, err := filepath.Rel(root, destination)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("archive entry escapes target: %s", name)
	}
	return destination, nil
}

// copyReaderToFile 把 reader 内容写入目标文件。
func copyReaderToFile(reader io.Reader, target string, mode os.FileMode) error {
	if mode == 0 {
		mode = 0o644
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, reader); err != nil {
		return err
	}
	return targetFile.Sync()
}

// archivePayloadDir 如果压缩包只有一个外层目录，则返回这个外层目录。
// 这样带外层目录和不带外层目录的压缩包，都能把有效内容落到目标目录。
func archivePayloadDir(extractDir string) (string, error) {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	if len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(extractDir, entries[0].Name()), nil
	}
	return extractDir, nil
}

// copyPath 复制文件或目录，用于备份。
func copyPath(source string, target string) error {
	sourceInfo, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 {
		return copySymlink(source, target)
	}
	if sourceInfo.IsDir() {
		return copyDirContents(source, target)
	}
	return copyFile(source, target)
}

// copyDirContents 把目录内容复制到目标目录。
func copyDirContents(source string, target string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return os.MkdirAll(target, 0o755)
		}

		destination := filepath.Join(target, relative)
		if entry.Type()&os.ModeSymlink != 0 {
			return copySymlink(path, destination)
		}
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			mode := info.Mode().Perm()
			if mode == 0 {
				mode = 0o755
			}
			if err := os.MkdirAll(destination, mode); err != nil {
				return err
			}
			return os.Chmod(destination, mode)
		}
		return copyFile(path, destination)
	})
}

func copySymlink(source string, target string) error {
	linkTarget, err := os.Readlink(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Symlink(linkTarget, target)
}

// copyFile 复制单个文件。
func copyFile(source string, target string) error {
	sourceFile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	targetFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, sourceInfo.Mode().Perm())
	if err != nil {
		return err
	}
	defer targetFile.Close()

	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return err
	}
	return targetFile.Sync()
}

// backupRelativePath 把绝对目标路径变成备份目录下的相对路径。
func backupRelativePath(target string) string {
	clean := filepath.Clean(target)
	volume := filepath.VolumeName(clean)
	clean = strings.TrimPrefix(clean, volume)
	clean = strings.TrimLeft(clean, string(filepath.Separator))

	if volume != "" {
		volume = strings.TrimSuffix(volume, ":")
		clean = filepath.Join(volume, clean)
	}
	if clean == "" {
		return filepath.Base(target)
	}
	return clean
}

func targetEndsWithSeparator(target string) bool {
	return strings.HasSuffix(target, "/") || strings.HasSuffix(target, "\\")
}

func isRootPath(path string) bool {
	volume := filepath.VolumeName(path)
	root := volume + string(filepath.Separator)
	return path == root
}

func isArchiveSource(source string) bool {
	lower := strings.ToLower(source)
	return strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar") ||
		strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz")
}

func pathWithin(root string, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func rejectSymlinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			relative, _ := filepath.Rel(root, path)
			return fmt.Errorf("deploy source contains symbolic link: %s", relative)
		}
		return nil
	})
}
