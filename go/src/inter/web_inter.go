package inter

import "context"

type WebServer interface {
	Start(ctx context.Context) error
}
