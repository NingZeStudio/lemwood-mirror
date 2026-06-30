package assets

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSyncEmbedded_ReleasesOnEveryRestart 验证每次启动都重新释放前端文件，
// 不再依赖 manifest 哈希短路。模拟：首次释放后删掉一个文件，再次同步应恢复。
func TestSyncEmbedded_ReleasesOnEveryRestart(t *testing.T) {
	dir := t.TempDir()

	if err := SyncEmbedded(dir); err != nil {
		t.Fatalf("首次 SyncEmbedded 失败: %v", err)
	}

	defaultIndex := filepath.Join(dir, "web", "default", "index.html")
	if _, err := os.Stat(defaultIndex); err != nil {
		t.Fatalf("首次释放后 web/default/index.html 应存在, got err=%v", err)
	}

	// 删除一个文件，模拟运行中被外部清理/损坏
	if err := os.Remove(defaultIndex); err != nil {
		t.Fatalf("删除 index.html 失败: %v", err)
	}

	// 再次同步：旧逻辑（manifest 短路）不会重新释放，新逻辑每次都释放
	if err := SyncEmbedded(dir); err != nil {
		t.Fatalf("第二次 SyncEmbedded 失败: %v", err)
	}

	if _, err := os.Stat(defaultIndex); err != nil {
		t.Fatalf("再次释放后 index.html 应恢复, got err=%v", err)
	}
}

// TestSyncEmbedded_OverwritesStaleContent 验证当本地文件内容与内嵌不一致时会被覆盖。
func TestSyncEmbedded_OverwritesStaleContent(t *testing.T) {
	dir := t.TempDir()

	if err := SyncEmbedded(dir); err != nil {
		t.Fatalf("首次 SyncEmbedded 失败: %v", err)
	}

	defaultIndex := filepath.Join(dir, "web", "default", "index.html")
	original, err := os.ReadFile(defaultIndex)
	if err != nil {
		t.Fatalf("读取原始内容失败: %v", err)
	}

	// 篡改本地内容
	if err := os.WriteFile(defaultIndex, []byte("<html>stale</html>"), 0o644); err != nil {
		t.Fatalf("写入篡改内容失败: %v", err)
	}

	// 再次同步应覆盖回内嵌内容
	if err := SyncEmbedded(dir); err != nil {
		t.Fatalf("第二次 SyncEmbedded 失败: %v", err)
	}

	current, err := os.ReadFile(defaultIndex)
	if err != nil {
		t.Fatalf("读取恢复内容失败: %v", err)
	}
	if string(current) != string(original) {
		t.Fatalf("内容未被覆盖回内嵌版本；期望与首次释放一致")
	}
}
