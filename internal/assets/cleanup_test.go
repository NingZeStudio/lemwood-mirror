package assets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanupDeprecatedBundles_RemovesDefaultV2(t *testing.T) {
	dir := t.TempDir()

	// 创建 web/default_v2/ 目录，含一个 dummy 文件
	v2Dir := filepath.Join(dir, "web", "default_v2")
	if err := os.MkdirAll(v2Dir, 0o755); err != nil {
		t.Fatalf("mkdir error = %v", err)
	}
	dummy := filepath.Join(v2Dir, "index.html")
	if err := os.WriteFile(dummy, []byte("<html>old</html>"), 0o644); err != nil {
		t.Fatalf("write dummy error = %v", err)
	}

	cleanupDeprecatedBundles(dir)

	if _, err := os.Stat(v2Dir); !os.IsNotExist(err) {
		t.Fatalf("expected web/default_v2 to be deleted, got err=%v", err)
	}
}

func TestCleanupDeprecatedBundles_NoOpWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	// 不创建 web/default_v2/

	// 应无错误返回（不存在则跳过）
	cleanupDeprecatedBundles(dir)

	// 验证目录确实不存在
	if _, err := os.Stat(filepath.Join(dir, "web", "default_v2")); !os.IsNotExist(err) {
		t.Fatalf("expected web/default_v2 to not exist, got err=%v", err)
	}
}

func TestCleanupDeprecatedBundles_SkipsNonDirectory(t *testing.T) {
	dir := t.TempDir()

	// 在 web/default_v2 路径放一个文件（非目录），模拟用户误放
	v2Path := filepath.Join(dir, "web", "default_v2")
	if err := os.MkdirAll(filepath.Dir(v2Path), 0o755); err != nil {
		t.Fatalf("mkdir parent error = %v", err)
	}
	if err := os.WriteFile(v2Path, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write file error = %v", err)
	}

	cleanupDeprecatedBundles(dir)

	// 保守策略：文件不应被删除
	info, err := os.Stat(v2Path)
	if err != nil {
		t.Fatalf("expected file preserved (conservative skip), got err=%v", err)
	}
	if info.IsDir() {
		t.Fatalf("expected file to remain a file, but is dir")
	}
}
