// 本文件提供 Remix IDE 静态资源的最小 HTTP 服务。
package main

import (
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const version = "remix-static-server-1"

// main 校验静态目录并启动只服务 Remix 静态资源的 HTTP 服务器。
func main() {
	addr := flag.String("addr", ":8080", "listen address")
	root := flag.String("root", "/usr/share/remix", "static root")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		log.Fatalf("resolve static root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(absRoot, "index.html")); err != nil {
		log.Fatalf("missing Remix index.html: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler(absRoot))

	server := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5_000_000_000,
	}
	log.Printf("serving Remix static assets on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve Remix static assets: %v", err)
	}
}

// staticHandler 返回静态资源处理器,并对前端路由回退到 index.html。
func staticHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		rel := strings.TrimPrefix(filepath.Clean("/"+r.URL.Path), string(filepath.Separator))
		target := filepath.Join(root, rel)
		if !strings.HasPrefix(target, root) {
			http.NotFound(w, r)
			return
		}

		info, err := os.Stat(target)
		if err != nil || info.IsDir() {
			target = filepath.Join(root, "index.html")
		}
		if contentType := mime.TypeByExtension(filepath.Ext(target)); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		http.ServeFile(w, r, target)
	})
}
