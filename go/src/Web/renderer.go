package Web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/aarondl/authboss/v3"
)

// HTMLRenderer adapts html/template to authboss.Renderer
type HTMLRenderer struct {
	templates map[string]*template.Template
	htmlDir   string
}

// NewHTMLRenderer creates a new renderer
func NewHTMLRenderer(htmlDir string) *HTMLRenderer {
	return &HTMLRenderer{
		templates: loadTemplates(htmlDir), // Reuse existing template loader
		htmlDir:   htmlDir,
	}
}

// Load reloads templates
func (r *HTMLRenderer) Load(names ...string) error {
	r.templates = loadTemplates(r.htmlDir)
	return nil
}

// Render renders the template
func (r *HTMLRenderer) Render(ctx context.Context, page string, data authboss.HTMLData) (output []byte, contentType string, err error) {
	tplName := page + ".html"
	t, ok := r.templates[tplName]
	if !ok {
		return nil, "text/html", fmt.Errorf("template not found for page: %s", page)
	}

	viewData := make(map[string]interface{})
	for k, v := range data {
		viewData[k] = v
	}

	// Adapt errors for our simple templates
	// Authboss puts validation errors in "errors" (map[string][]string)
	if errs, ok := data["errors"].([]string); ok && len(errs) > 0 {
		viewData["Error"] = errs[0]
	} else if errMap, ok := data["errors"].(map[string][]string); ok {
		for _, v := range errMap {
			if len(v) > 0 {
				viewData["Error"] = v[0]
				break
			}
		}
	}
	// Authboss puts general flash errors in "error" or "flash_error" depending on config
	if errStr, ok := data["error"].(string); ok {
		viewData["Error"] = errStr
	} else if flashErr, ok := data["flash_error"].(string); ok {
		viewData["Error"] = flashErr
	}

	// Flash Success
	if successMsg, ok := data["flash_success"].(string); ok {
		viewData["Success"] = successMsg
	}

	var buf bytes.Buffer

	if err := t.Execute(&buf, viewData); err != nil {
		return nil, "text/html", err
	}

	return buf.Bytes(), "text/html", nil
}
