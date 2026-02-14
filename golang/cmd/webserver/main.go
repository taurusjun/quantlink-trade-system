package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 8080, "HTTP server port")
	dir := flag.String("dir", "", "Directory to serve (default: auto-detect golang/web)")
	flag.Parse()

	// Auto-detect web directory
	webDir := *dir
	if webDir == "" {
		// Try relative paths from typical working directories
		candidates := []string{
			"golang/web",
			"web",
			filepath.Join(filepath.Dir(os.Args[0]), "..", "golang", "web"),
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && info.IsDir() {
				webDir = c
				break
			}
		}
		if webDir == "" {
			log.Fatal("Cannot find web directory. Use -dir flag.")
		}
	}

	// CORS middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Serve overview.html as index
		if r.URL.Path == "/" || r.URL.Path == "/overview" {
			http.ServeFile(w, r, filepath.Join(webDir, "overview.html"))
			return
		}

		http.FileServer(http.Dir(webDir)).ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("[WebServer] Serving %s on http://%s", webDir, addr)
	log.Printf("[WebServer] Overview: http://localhost:%d/", *port)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("[WebServer] %v", err)
	}
}
