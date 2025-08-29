package main

import (
	"archive/zip"
	"bytes"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	addr     = flag.String("addr", ":8080", "listen address, e.g. :8080 or 127.0.0.1:8080")
	root     = flag.String("dir", ".", "root directory to share")
	maxMB    = flag.Int("maxmb", 1024, "max upload size in megabytes")
	password = flag.String("password", "password", "password for access")
)

//go:embed static/htmx.min.js
var htmx string

//go:embed static/upload.js
var uploadJS string

//go:embed static/favicon.svg
var faviconSVG string

//go:embed static/download.svg
var downloadSVG string

//go:embed static/delete.svg
var deleteSVG string

//go:embed static/upload.svg
var uploadSVG string

//go:embed static/folder.svg
var folderSVG string

//go:embed static/file.svg
var fileSVG string

//go:embed tmpl/login.html
var loginHTML string

// Embed ensures the variables above are populated; unused check
var _ = embed.FS{}

func main() {
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		log.Fatalf("invalid dir: %v", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		log.Fatalf("create dir: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/login", loginHandler())
	mux.HandleFunc("/logout", logoutHandler)
	mux.Handle("/files", requireLogin(filesHandler()))

	mux.Handle("/view/{fileName...}", requireLogin(FileViewHandler(absRoot)))

	// All file routes require login
	mux.Handle("/", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Title string
			MaxMB int
			Files []FileInfo
		}{
			Title: "Files - File Share",
			MaxMB: *maxMB,
		}

		data.Files, err = GetFiles(*root)
		if err != nil {
			http.Error(w, fmt.Sprintf("get files error: %v", err), http.StatusInternalServerError)
			return
		}

		serveHtmlTemplate(w, "./tmpl/index.html", "Files", data)
	})))

	mux.Handle("/header", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Now time.Time
		}{
			Now: time.Now(),
		}

		serveHtmlTemplate(w, "./tmpl/header.html", "header", data)
	})))

	mux.Handle("/upload", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			data := struct {
				Title string
				MaxMB int
			}{
				Title: "Upload Files - Sharge",
				MaxMB: *maxMB,
			}

			serveHtmlTemplate(w, "./tmpl/upload.html", "Upload Files", data)
			return
		}

		if r.Method == http.MethodPost {
			r.Body = http.MaxBytesReader(w, r.Body, int64(*maxMB)<<20)
			if err := r.ParseMultipartForm(int64(*maxMB) << 20); err != nil {
				http.Error(w, fmt.Sprintf("parse multipart: %v", err), http.StatusBadRequest)
				return
			}
			files := r.MultipartForm.File["files[]"]
			if len(files) == 0 {
				http.Error(w, "no files", http.StatusBadRequest)
				return
			}
			for _, fh := range files {
				if err := saveUploadedFile(absRoot, fh); err != nil {
					log.Printf("upload error for %q: %v", fh.Filename, err)
					http.Error(w, fmt.Sprintf("save %s: %v", fh.Filename, err), http.StatusBadRequest)
					return
				}

				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte("ok"))
			}
		}
	})))

	mux.Handle("/dl", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("f")
		if name == "" {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}

		files := r.URL.Query()["f"]
		if len(files) == 1 {
			p := filepath.Join(absRoot, name)
			if !within(absRoot, p) {
				http.Error(w, "invalid path", http.StatusBadRequest)
				return
			}

			fi, err := os.Stat(p)
			if err != nil || fi.IsDir() {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", fi.Name()))
			http.ServeFile(w, r, p)
			return

		} else {

			buf := new(bytes.Buffer)
			zipWriter := zip.NewWriter(buf)

			for _, file := range files {
				p := filepath.Join(absRoot, file)
				if !within(absRoot, p) {
					http.Error(w, "invalid path", http.StatusBadRequest)
					return
				}

				fi, err := os.Stat(p)
				if err != nil || fi.IsDir() {
					http.NotFound(w, r)
					return
				}

				fileWriter, err := zipWriter.Create(file)
				if err != nil {
					http.Error(w, fmt.Sprintf("zipWriter.Create error %s", err), http.StatusInternalServerError)
					return
				}

				file, err := os.Open(p)
				if err != nil {
					http.Error(w, fmt.Sprintf("os.Open error %s", err), http.StatusInternalServerError)
					return
				}
				defer file.Close()

				_, err = io.Copy(fileWriter, file)
				if err != nil {
					http.Error(w, fmt.Sprintf("io.Copy error %s", err), http.StatusInternalServerError)
					return
				}
			}

			zipWriter.Close()

			zipBytes := buf.Bytes()

			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", "sharge.zip"))
			w.Header().Set("Content-Type", "application/zip")
			_, err = w.Write(zipBytes)
			if err != nil {
				http.Error(w, fmt.Sprintf("w.Write error %s", err), http.StatusInternalServerError)
				return
			}
		}
	})))

	mux.Handle("/mkdir", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("d")
		if name == "" {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}

		p := filepath.Join(absRoot, name)
		if !within(absRoot, p) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		err := os.MkdirAll(p, os.ModeDir)
		if err != nil {
			http.Error(w, fmt.Sprintf("os.MkdirAll error %s", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
	})))

	mux.Handle("/rm", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := sanitizeName(r.URL.Query().Get("f"))
		if name == "" {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}

		p := filepath.Join(absRoot, name)
		if !within(absRoot, p) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		if err := os.Remove(p); err != nil {
			http.Error(w, fmt.Sprintf("remove: %v", err), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusOK)
		//w.Header().Add("HX-Trigger", fmt.Sprintf(`{ "showMessage": "File %s deleted successfuly!" }`, name))
	})))

	mux.Handle("/re", requireLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		oldName := r.URL.Query().Get("o")
		if oldName == "" {
			http.Error(w, "missing old path", http.StatusBadRequest)
			return
		}

		newName := r.URL.Query().Get("n")
		if newName == "" {
			http.Error(w, "missing new path", http.StatusBadRequest)
			return
		}

		oldp := filepath.Join(absRoot, oldName)
		if !within(absRoot, oldp) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		newp := filepath.Join(absRoot, newName)
		if !within(absRoot, newp) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		// Create the new parent directory if it doesn't exist
		dir := filepath.Dir(newp)
		err = os.MkdirAll(dir, 0755) // Use MkdirAll for nested directories
		if err != nil {
			http.Error(w, fmt.Sprintf("MkdirAll error: %v", err), http.StatusInternalServerError)
			fmt.Println("Error creating new directory:", err)
			return
		}

		if err := os.Rename(oldp, newp); err != nil {
			http.Error(w, fmt.Sprintf("Rename error: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		//w.Header().Add("HX-Trigger", fmt.Sprintf(`{ "showMessage": "File %s deleted successfuly!" }`, name))
	})))

	mux.HandleFunc("/output.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		styleCSS, err := os.ReadFile("./dist/output.css")
		if err != nil {
			log.Printf("./dist/output.css not found")
		}
		_, _ = io.Writer.Write(w, styleCSS)
	})

	mux.HandleFunc("/static/htmx.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, htmx)
	})

	mux.HandleFunc("/static/upload.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = io.WriteString(w, uploadJS)
	})

	mux.HandleFunc("/static/toast.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		toastTJ, err := os.ReadFile("./static/toast.js")
		if err != nil {
			log.Printf("./static/toast.js not found")
		}
		_, _ = io.Writer.Write(w, toastTJ)
	})

	mux.HandleFunc("/static/filetree.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		js, err := os.ReadFile("./static/filetree.js")
		if err != nil {
			log.Printf("./static/filetree.js not found")
		}
		_, _ = io.Writer.Write(w, js)
	})

	mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, faviconSVG)
	})
	mux.HandleFunc("/icons/download.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, downloadSVG)
	})
	mux.HandleFunc("/icons/delete.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, deleteSVG)
	})
	mux.HandleFunc("/icons/upload.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, uploadSVG)
	})
	mux.HandleFunc("/icons/folder.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, folderSVG)
	})
	mux.HandleFunc("/icons/file.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		_, _ = io.WriteString(w, fileSVG)
	})

	log.Printf("Serving %s on http://%s (max=%dMB)", absRoot, *addr, *maxMB)
	if err := http.ListenAndServe(*addr, logRequests(mux)); err != nil {
		log.Fatal(err)
	}
}

// Simple request logging middleware
func logRequests(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		h.ServeHTTP(w, r)
		log.Printf("%s %s %s %s", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})
}

func serveHtmlTemplate(w http.ResponseWriter, path string, title string, data any) {
	page, err := os.ReadFile(path)
	if err != nil {
		log.Printf("%s not found", path)
	}

	tmpl := template.Must(template.New(title).Parse(string(page)))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("template execute: %v", err)
	}
}

func filesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		files, err := GetFiles(*root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		data := struct {
			Files   []FileInfo
			DirPath string
			Parent  string
		}{
			Files:   files,
			DirPath: "",
			Parent:  "",
		}

		serveHtmlTemplate(w, "./tmpl/files.html", "Files", data)
	}
}

type FileInfo struct {
	Path     string
	Name     string
	Size     string
	Modified time.Time
	IsDir    bool
	Children []FileInfo // for directories
}

func GetFiles(relativeDir string) ([]FileInfo, error) {
	abs, err := filepath.Abs(relativeDir)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			continue
		}

		var sizeStr string
		if e.IsDir() {
			sizeStr = "-"
		} else if fi.Size() < 1024 {
			sizeStr = fmt.Sprintf("%d B", fi.Size())
		} else if fi.Size() < 1024*1024 {
			sizeStr = fmt.Sprintf("%.1f KB", float64(fi.Size())/1024)
		} else if fi.Size() < 1024*1024*1024 {
			sizeStr = fmt.Sprintf("%.1f MB", float64(fi.Size())/float64((1024*1024)))
		} else {
			sizeStr = fmt.Sprintf("%.1f GB", float64(fi.Size())/float64(1024*1024*1024))
		}

		var children []FileInfo
		if e.IsDir() {
			childPath := filepath.Join(relativeDir, e.Name())
			children, _ = GetFiles(childPath)
		}

		path := strings.ReplaceAll(relativeDir, `\`, `/`)
		path = strings.TrimPrefix(path, *root)
		path = path + "/" + e.Name()
		path = strings.TrimPrefix(path, "/")

		files = append(files, FileInfo{
			Path:     path,
			Name:     e.Name(),
			Size:     sizeStr,
			Modified: fi.ModTime(),
			IsDir:    e.IsDir(),
			Children: children,
		})
	}

	// Sort: directories first, then files, both by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})
	return files, nil
}

func sanitizeName(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.Trim(name, ". ") // trim dots/spaces
	if name == "." || name == ".." {
		return ""
	}
	return name
}

func within(root, p string) bool {
	ap, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	return strings.HasPrefix(ap+string(filepath.Separator), root+string(filepath.Separator))
}

func saveUploadedFile(root string, fh *multipart.FileHeader) error {
	name := sanitizeName(fh.Filename)
	if name == "" {
		return fmt.Errorf("invalid filename")
	}

	// If file exists, append a counter
	path := filepath.Join(root, name)
	if !within(root, path) {
		return fmt.Errorf("invalid path")
	}

	base := name
	ext := ""
	if dot := strings.LastIndex(name, "."); dot > 0 {
		base, ext = name[:dot], name[dot:]
	}

	for i := 1; fileExists(path); i++ {
		path = filepath.Join(root, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}

	src, err := fh.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return err
	}

	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
