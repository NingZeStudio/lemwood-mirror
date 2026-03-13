package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SPAFallbackMiddleware 为静态资源目录提供 SPA fallback 支持
// 适用于 Vite createWebHistory 模式
// - 跳过 /api/ 开头的路由
// - 精确匹配文件：如果请求路径对应真实文件，直接返回
// - 精确匹配目录：如果请求路径对应目录，返回 404（不列出目录）
// - 其他情况返回 index.html，交由前端路由处理
func SPAFallbackMiddleware(next http.Handler, staticDir string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// 跳过 /api/ 路由
		if strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// 跳过 /admin 路由（包括 /admin 和 /admin/）
		if path == "/admin" || strings.HasPrefix(path, "/admin/") {
			next.ServeHTTP(w, r)
			return
		}

		// 跳过 /download/ 路由
		if strings.HasPrefix(path, "/download/") {
			next.ServeHTTP(w, r)
			return
		}

		// 跳过 /dist/ 和 /assets/ 路由（已有独立处理）
		if strings.HasPrefix(path, "/dist/") || strings.HasPrefix(path, "/assets/") {
			next.ServeHTTP(w, r)
			return
		}

		// 清理路径
		relPath := strings.TrimPrefix(path, "/")
		if relPath == "" {
			relPath = "."
		}

		fullPath := filepath.Join(staticDir, relPath)
		cleanPath := filepath.Clean(fullPath)

		// 安全检查：确保路径在 staticDir 内
		absBase, _ := filepath.Abs(staticDir)
		absPath, _ := filepath.Abs(cleanPath)
		if !strings.HasPrefix(absPath, absBase) {
			log.Printf("安全警告：拦截到来自 %s 的路径逃逸尝试，请求路径：%s", r.RemoteAddr, path)
			http.NotFound(w, r)
			return
		}

		// 检查路径是否存在
		info, err := os.Stat(cleanPath)
		if err == nil {
			// 路径存在
			if info.IsDir() {
				// 精确匹配目录：不列出目录内容，返回 index.html（SPA fallback）
				indexPath := filepath.Join(staticDir, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return
				}
				http.NotFound(w, r)
				return
			}
			// 精确匹配文件：直接返回文件内容
			http.ServeFile(w, r, cleanPath)
			return
		}

		// 路径不存在（文件或目录都不存在）
		// fallback 到 index.html，交由前端路由处理
		indexPath := filepath.Join(staticDir, "index.html")
		if _, err := os.Stat(indexPath); err == nil {
			http.ServeFile(w, r, indexPath)
			return
		}

		// 连 index.html 都没有，返回 404
		http.NotFound(w, r)
	})
}
