package web

import (
	"context"

	"github.com/aarondl/authboss/v3"
)

// StaticViewRenderer 用于纯 API 模式下满足 authboss 的渲染依赖。
type StaticViewRenderer struct{}

func NewStaticViewRenderer() *StaticViewRenderer {
	return &StaticViewRenderer{}
}

func (r *StaticViewRenderer) Load(names ...string) error {
	return nil
}

func (r *StaticViewRenderer) Render(_ context.Context, page string, _ authboss.HTMLData) ([]byte, string, error) {
	body := "<!doctype html><html><body>" + page + " is not supported on this server.</body></html>"
	return []byte(body), "text/html; charset=utf-8", nil
}
