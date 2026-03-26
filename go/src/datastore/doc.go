// Package datastore 保留 legacy SQL 全量仓储实现。
//
// 新代码应优先通过 persistence.OpenAuthStore 或 persistence.OpenRuntimeStore
// 获取认证存储和运行时存储，不再直接依赖本包。
package datastore
