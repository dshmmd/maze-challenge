// Package webui serves a single-page console so a reviewer can exercise every
// market feature in a browser without curl. It is a thin client over the JSON
// API: the Go template only injects the seeded guild list; all actions are
// fetch() calls carrying the X-Guild-Id header.
package webui

import (
	_ "embed"
	"html/template"
	"net/http"
)

//go:embed index.html
var indexHTML string

// Handler renders the console page.
type Handler struct {
	tmpl   *template.Template
	guilds []string
}

// New builds the UI handler with the guild IDs offered in the switcher.
func New(guilds []string) (*Handler, error) {
	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		return nil, err
	}
	return &Handler{tmpl: tmpl, guilds: guilds}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = h.tmpl.Execute(w, struct{ Guilds []string }{Guilds: h.guilds})
}
