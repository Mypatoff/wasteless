// Package web embeds the HTML templates and static assets into the
// binary, so the server is a single file with no separate folders to ship.
package web

import (
	"embed"
	"html/template"
	"io/fs"
)

// go:embed can't reach outside this directory (web/), which is why
// templates and static must live here rather than at the project root.
//
//go:embed templates static
var files embed.FS

// Page parses layout.html together with exactly one page's content
// template. Parsing every page template in one pass would make their
// {{define "content"}} blocks collide (the last one parsed would win for
// every page), so each page gets its own fresh *template.Template.
func Page(name string) (*template.Template, error) {
	return template.ParseFS(files, "templates/layout.html", "templates/"+name+".html")
}

// StaticFS returns the embedded static assets rooted at "static", so they
// can be served at /static/... without that prefix leaking into the path.
func StaticFS() (fs.FS, error) {
	return fs.Sub(files, "static")
}
