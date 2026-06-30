package assets

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	rootassets "lemwood_mirror"
)

type Bundle struct {
	Subdir string
	Target string
}

// deprecatedBundles 是历史上 SyncEmbedded 曾同步、但当前版本不再维护的 bundle
// Target 目录（相对 projectRoot）。升级时若检测到这些目录存在，会整体删除以
// 避免遗留前端构建产物。未来若有新的废弃 bundle，追加到此列表即可。
var deprecatedBundles = []string{
	"web/default_v2", // 6月24日 commit 0343b4c 移除双主题后遗留
}

func SyncEmbedded(projectRoot string) error {
	bundles := []Bundle{
		{Subdir: "web/default", Target: filepath.Join(projectRoot, "web", "default")},
		{Subdir: "web/admin", Target: filepath.Join(projectRoot, "web", "admin")},
	}

	for _, bundle := range bundles {
		if err := syncBundle(bundle); err != nil {
			return err
		}
	}

	cleanupDeprecatedBundles(projectRoot)
	return nil
}

// cleanupDeprecatedBundles 删除历史上曾生成、但当前版本不再需要的 bundle 目录。
// 仅当目录存在时尝试删除；失败仅记录日志，不阻断启动。
func cleanupDeprecatedBundles(projectRoot string) {
	for _, rel := range deprecatedBundles {
		target := filepath.Join(projectRoot, filepath.FromSlash(rel))
		info, err := os.Stat(target)
		if err != nil {
			continue // 不存在，跳过
		}
		if !info.IsDir() {
			continue // 不是目录（可能是用户误放的文件），保守不删
		}
		if err := os.RemoveAll(target); err != nil {
			log.Printf("[资源清理] 删除遗留目录 %s 失败（可手动删除）: %v", target, err)
		} else {
			log.Printf("[资源清理] 已删除遗留目录 %s", target)
		}
	}
}

func syncBundle(bundle Bundle) error {
	source, err := fs.Sub(rootassets.EmbeddedFiles, bundle.Subdir)
	if err != nil {
		return fmt.Errorf("读取嵌入资源 %s 失败: %w", bundle.Subdir, err)
	}

	if err := os.MkdirAll(bundle.Target, 0o755); err != nil {
		return fmt.Errorf("创建资源目录失败: %w", err)
	}
	// 每次启动都重新释放；extractFS 内部对内容相同的文件跳过写入，避免无谓 IO。
	// 不再保留 manifest 哈希短路，确保前端文件随二进制更新即时生效。
	if err := extractFS(source, bundle.Target); err != nil {
		return err
	}
	// 清理旧版本遗留的 .embedded-manifest 文件（已不再使用）。
	_ = os.Remove(filepath.Join(bundle.Target, ".embedded-manifest"))
	return nil
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
