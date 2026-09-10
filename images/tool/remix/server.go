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
	"strconv"
	"strings"
	"time"
)

const version = "remix-static-server-1"

// main 校验静态目录并启动只服务 Remix 静态资源的 HTTP 服务器。
func main() {
	addr := flag.String("addr", "", "listen address")
	root := flag.String("root", "", "static root")
	showVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}
	if strings.TrimSpace(*addr) == "" || strings.TrimSpace(*root) == "" {
		log.Fatal("-addr and -root are required")
	}
	httpTimeout := time.Duration(requiredPositiveInt("CHAIMIR_HTTP_TIMEOUT_SECONDS")) * time.Second

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
		ReadHeaderTimeout: httpTimeout,
		ReadTimeout:       httpTimeout,
		WriteTimeout:      httpTimeout,
		IdleTimeout:       httpTimeout,
		MaxHeaderBytes:    16 << 10,
	}
	log.Printf("serving Remix static assets on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("serve Remix static assets: %v", err)
	}
}

// requiredPositiveInt 读取服务端有限的正整数配置。
func requiredPositiveInt(key string) int {
	value := strings.TrimSpace(os.Getenv(key))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed
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
		relativeTarget, err := filepath.Rel(root, target)
		if err != nil || relativeTarget == ".." || strings.HasPrefix(relativeTarget, ".."+string(filepath.Separator)) {
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
