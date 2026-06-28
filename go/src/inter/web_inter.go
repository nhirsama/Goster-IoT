package inter

import (
	"context"
	"net"
)

type WebServer interface {
	Start(ctx context.Context) error
	Serve(ctx context.Context, listener net.Listener) error
}
