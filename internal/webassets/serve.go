package webassets

import (
	"bytes"
	"io/fs"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

// SubStatic 返回嵌入的 static/ 子目录（index.html 位于根）。
func SubStatic() (fs.FS, error) {
	return fs.Sub(Static, "static")
}

// SPAHandler 为 Vue hash 路由提供静态文件与 index.html 回退。
func SPAHandler(root fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
		if name == "." || name == "" {
			name = "index.html"
		}
		data, err := fs.ReadFile(root, name)
		if err != nil {
			data, err = fs.ReadFile(root, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}
		ct := mime.TypeByExtension(filepath.Ext(name))
		if ct == "" {
			ct = "application/octet-stream"
		}
		w.Header().Set("Content-Type", ct)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})
}

// HasIndex 检查嵌入资源是否包含 index.html。
func HasIndex() bool {
	if !Enabled() {
		return false
	}
	_, err := Static.Open("static/index.html")
	return err == nil
}
