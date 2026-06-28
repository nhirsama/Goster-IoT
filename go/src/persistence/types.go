package persistence

import "github.com/nhirsama/Goster-IoT/src/inter"

// RuntimeStore 是当前业务运行时依赖的最小仓储组合。
// 认证链路使用独立的 identity.Store，不再复用全量仓储。
type RuntimeStore interface {
	inter.CoreStore
	inter.WebV1Store
}

// CloseIfPossible 在存储实现提供 Close 方法时执行资源释放。
func CloseIfPossible(store any) error {
	closer, ok := store.(interface{ Close() error })
	if !ok {
		return nil
	}
	return closer.Close()
}
