package assets

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	rootassets "lemwood_mirror"
)

const manifestFile = ".embedded-manifest"

type Bundle struct {
	Subdir string
	Target string
}

func SyncEmbedded(projectRoot string) error {
	bundles := []Bundle{
		{Subdir: "web/default", Target: filepath.Join(projectRoot, "web", "default")},
		{Subdir: "web/default_v2", Target: filepath.Join(projectRoot, "web", "default_v2")},
		{Subdir: "web/admin", Target: filepath.Join(projectRoot, "web", "admin")},
	}

	for _, bundle := range bundles {
		if err := syncBundle(bundle); err != nil {
			return err
		}
	}

	return nil
}

func syncBundle(bundle Bundle) error {
	source, err := fs.Sub(rootassets.EmbeddedFiles, bundle.Subdir)
	if err != nil {
		return fmt.Errorf("读取嵌入资源 %s 失败: %w", bundle.Subdir, err)
	}

	manifest, err := buildManifest(source)
	if err != nil {
		return fmt.Errorf("构建嵌入资源摘要失败: %w", err)
	}

	currentManifest, err := os.ReadFile(filepath.Join(bundle.Target, manifestFile))
	if err == nil && strings.TrimSpace(string(currentManifest)) == manifest {
		return nil
	}
	if err := os.MkdirAll(bundle.Target, 0o755); err != nil {
		return fmt.Errorf("创建资源目录失败: %w", err)
	}
	if err := extractFS(source, bundle.Target); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(bundle.Target, manifestFile), []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("写入资源摘要失败: %w", err)
	}
	return nil
}

func buildManifest(root fs.FS) (string, error) {
	hasher := sha256.New()
	var paths []string
	if err := fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return "", err
	}
	sort.Strings(paths)
	for _, path := range paths {
		b, err := fs.ReadFile(root, path)
		if err != nil {
			return "", err
		}
		hasher.Write([]byte(path))
		hasher.Write([]byte{0})
		hasher.Write(b)
		hasher.Write([]byte{0})
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func extractFS(root fs.FS, target string) error {
	return fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		fullPath := filepath.Join(target, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(fullPath, 0o755)
		}
		content, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		existing, err := os.ReadFile(fullPath)
		if err == nil && bytes.Equal(existing, content) {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(fullPath, content, 0o644)
	})
}
