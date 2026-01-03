package Web

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

	"github.com/aarondl/authboss/v3"
)

// HTMLRenderer 适配 html/template 到 authboss.Renderer 接口
type HTMLRenderer struct {
	templates map[string]*template.Template
	htmlDir   string
}

// NewHTMLRenderer 创建一个新的 HTML 渲染器
func NewHTMLRenderer(htmlDir string) *HTMLRenderer {
	return &HTMLRenderer{
		templates: loadTemplates(htmlDir),
		htmlDir:   htmlDir,
	}
}

// Load 重新加载模板
func (r *HTMLRenderer) Load(names ...string) error {
	r.templates = loadTemplates(r.htmlDir)
	return nil
}

// Render 渲染指定的模板页面
func (r *HTMLRenderer) Render(ctx context.Context, page string, data authboss.HTMLData) (output []byte, contentType string, err error) {
	tplName := page + ".html"
	t, ok := r.templates[tplName]
	if !ok {
		return nil, "text/html", fmt.Errorf("模板未找到: %s", page)
	}

	viewData := make(map[string]interface{})
	for k, v := range data {
		viewData[k] = v
	}

	// 适配 Authboss 错误信息到模板
	// 验证错误通常在 "errors" (map[string][]string)
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
	// 通用 Flash 错误在 "error" 或 "flash_error"
	if errStr, ok := data["error"].(string); ok {
		viewData["Error"] = errStr
	} else if flashErr, ok := data["flash_error"].(string); ok {
		viewData["Error"] = flashErr
	}

	// Flash 成功消息
	if successMsg, ok := data["flash_success"].(string); ok {
		viewData["Success"] = successMsg
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, viewData); err != nil {
		return nil, "text/html", err
	}

	return buf.Bytes(), "text/html", nil
}
