package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

const version = "remix-static-server-1"

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

	assets, err := os.OpenRoot(*root)
	if err != nil {
		log.Fatalf("open static root: %v", err)
	}
	defer assets.Close()
	if info, err := assets.Stat("index.html"); err != nil || !info.Mode().IsRegular() {
		log.Fatalf("missing Remix index.html: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/", staticHandler(assets))

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

func requiredPositiveInt(key string) int {
	value := strings.TrimSpace(os.Getenv(key))
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		log.Fatalf("%s must be a positive integer", key)
	}
	return parsed
}

func staticHandler(root *os.Root) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		file, err := root.Open(name)
		// 仅前端导航回退；缺失资源或越界链接不能伪装成成功的 HTML。
		if errors.Is(err, os.ErrNotExist) && path.Ext(name) == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			file, err = root.Open("index.html")
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, info.Name(), info.ModTime(), file)
	})
}
