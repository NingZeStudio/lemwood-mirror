package gitmirror

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

var launcherNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func RepoDir(projectRoot, launcher string) (string, error) {
	if !launcherNamePattern.MatchString(launcher) {
		return "", fmt.Errorf("无效的 launcher 名称 %q", launcher)
	}

	repoBase := filepath.Join(projectRoot, "repo")
	repoDir := filepath.Join(repoBase, launcher+".git")

	absBase, err := filepath.Abs(repoBase)
	if err != nil {
		return "", fmt.Errorf("解析 repo 基础目录失败: %w", err)
	}
	absRepo, err := filepath.Abs(repoDir)
	if err != nil {
		return "", fmt.Errorf("解析 repo 目录失败: %w", err)
	}
	rel, err := filepath.Rel(absBase, absRepo)
	if err != nil {
		return "", fmt.Errorf("校验 repo 目录失败: %w", err)
	}
	if rel == "." || rel == ".." || len(rel) >= 3 && rel[:3] == ".."+string(os.PathSeparator) {
		return "", fmt.Errorf("repo 目录越界: %q", launcher)
	}

	return repoDir, nil
}

func Sync(ctx context.Context, projectRoot, launcher, repoURL string) error {
	repoDir, err := RepoDir(projectRoot, launcher)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(repoDir), 0o755); err != nil {
		return fmt.Errorf("创建 repo 目录失败: %w", err)
	}

	if _, err := os.Stat(filepath.Join(repoDir, "HEAD")); err == nil {
		if err := runGit(ctx, projectRoot, "-C", repoDir, "remote", "set-url", "origin", repoURL); err != nil {
			return err
		}
		if err := runGit(ctx, projectRoot, "-C", repoDir, "remote", "update", "--prune"); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		if err := runGit(ctx, projectRoot, "clone", "--mirror", repoURL, repoDir); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("检查 repo 目录失败: %w", err)
	}

	if err := runGit(ctx, projectRoot, "-C", repoDir, "update-server-info"); err != nil {
		return err
	}

	return nil
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v 失败: %w: %s", args, err, string(output))
	}
	return nil
}
