package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

func FileViewHandler(absRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("fileName")
		if name == "" {
			http.Error(w, "missing file", http.StatusBadRequest)
			return
		}

		p := filepath.Join(absRoot, name)
		if !within(absRoot, p) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		ServeFileView(w, r, p)
	}
}

func ServeFileView(w http.ResponseWriter, r *http.Request, p string) {
	fi, err := os.Stat(p)
	if err != nil || fi.IsDir() {
		http.NotFound(w, r)
		return
	}

	fileType, err := GetFileType(p)
	if err != nil {
		http.Error(w, fmt.Sprintf("GetFileType error %s\n", err), http.StatusBadRequest)
		return
	}

	switch fileType {
	case Video:
		StreamVideoFile(w, r, p, fi)
	case Audio:
		StreamVideoFile(w, r, p, fi)
	case Text:
		ServeTextFile(w, r, p)
	case Markdown:
		ServeMarkdownFile(w, r, p, fi)
	}
}

func StreamVideoFile(w http.ResponseWriter, r *http.Request, p string, fileInfo os.FileInfo) {
	file, err := os.Open(p)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	http.ServeContent(w, r, fileInfo.Name(), fileInfo.ModTime(), file)
}

func ServeTextFile(w http.ResponseWriter, r *http.Request, p string) {
	http.ServeFile(w, r, p)
}

func ServeMarkdownFile(w http.ResponseWriter, r *http.Request, p string, fileInfo os.FileInfo) {
	md, err := os.ReadFile(p)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	parser := parser.NewWithExtensions(extensions)
	doc := parser.Parse(md)

	// create HTML renderer with extensions
	htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.CompletePage
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	rendered := markdown.Render(doc, renderer)

	data := struct {
		Title    string
		Markdown template.HTML
	}{
		Title:    fileInfo.Name(),
		Markdown: template.HTML(string(rendered)),
	}

	serveHtmlTemplate(w, "./tmpl/markdown.html", "./tmpl/markdown.html", data)
}

type SupportedFileTypes int

const (
	None SupportedFileTypes = iota
	Video
	Audio
	Text
	Markdown
)

const SupporterVideoTypes string = "mp4 webm"
const SupporterAudioTypes string = "mp3"

func GetFileType(p string) (SupportedFileTypes, error) {
	dotIndex := strings.LastIndex(p, ".")
	if dotIndex == -1 {
		return None, fmt.Errorf("cant determine file type")
	}

	ext := p[dotIndex+1:]

	videoTypes := strings.Split(SupporterVideoTypes, " ")
	for _, videoExt := range videoTypes {
		if videoExt == ext {
			return Video, nil
		}
	}

	audioTypes := strings.Split(SupporterAudioTypes, " ")
	for _, audioExt := range audioTypes {
		if audioExt == ext {
			return Audio, nil
		}
	}

	if ext == "md" {
		return Markdown, nil
	}

	return Text, nil
}
