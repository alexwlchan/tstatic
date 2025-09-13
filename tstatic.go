package main

import (
	"flag"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"tailscale.com/tsnet"
)

var (
	dir = flag.String("dir", ".", "directory to serve")
)

func isBinaryFile(path string) bool {
	ext := filepath.Ext(path)

	switch mimeType := mime.TypeByExtension(ext); mimeType {
	case "image/gif", "image/jpeg", "image/png", "image/webp", "video/mp4", "video/x-m4v":
		return true
	default:
		return false
	}
}

func main() {
	flag.Parse()

	info, err := os.Stat(*dir)
	if err != nil || !info.IsDir() {
		log.Fatalf("Invalid directory: %s", *dir)
	}

	srv := new(tsnet.Server)
	defer srv.Close()

	ln, err := srv.Listen("tcp", ":80")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	lc, err := srv.LocalClient()
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.Dir(*dir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
			log.Printf("user=%s, node=%s, path=%s, range=%s",
				who.UserProfile.LoginName,
				firstLabel(who.Node.ComputedName),
				r.URL,
				rangeHeader)
		} else {
			log.Printf("user=%s, node=%s, path=%s",
				who.UserProfile.LoginName,
				firstLabel(who.Node.ComputedName),
				r.URL)
		}

		// Binary files like images and videos can be cached for a year,
		// because they basically never change.
		if isBinaryFile(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000")
		}

		fs.ServeHTTP(w, r)
	})

	log.Printf("Serving directory %q over tsnet", *dir)
	log.Fatal(http.Serve(ln, handler))
}

func firstLabel(s string) string {
	s, _, _ = strings.Cut(s, ".")
	return s
}
