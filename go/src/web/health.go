package web

import (
	"encoding/json"
	"net/http"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type healthResponse struct {
	Status string `json:"status"`
}

type readinessResponse struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// healthCheckHandler 返回服务的基本健康状态
// 用于负载均衡器检测服务是否存活
func (ws *webServer) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
}

// readinessCheckHandler 检查服务是否准备好接受流量
// 包括数据库连接等依赖项检查
func (ws *webServer) readinessCheckHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// 检查数据库连接（简单的查询测试）
	if err := ws.checkDatabaseConnection(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(readinessResponse{
			Status: "unavailable",
			Reason: "database unreachable",
		})
		ws.log().Warn("就绪检查失败：数据库不可达", inter.Err(err))
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(readinessResponse{Status: "ready"})
}

// checkDatabaseConnection 执行简单的数据库查询以验证连接
func (ws *webServer) checkDatabaseConnection() error {
	// 尝试查询用户数量作为健康检查
	_, err := ws.dataStore.GetUserCount()
	return err
}
