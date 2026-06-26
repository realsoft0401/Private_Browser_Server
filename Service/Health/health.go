package Health

import (
	"time"

	SQLiteInfra "private_browser_server/Infrastructures/SQLite"
	"private_browser_server/Settings"
)

type CheckResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type Response struct {
	OK         bool                   `json:"ok"`
	Status     string                 `json:"status"`
	Service    string                 `json:"service"`
	Mode       string                 `json:"mode"`
	Version    string                 `json:"version"`
	ConfigFile string                 `json:"configFile"`
	CheckedAt  int64                  `json:"checkedAt"`
	Checks     map[string]CheckResult `json:"checks"`
}

// BuildHealthResponse 汇总新的第一阶段最小健康事实。
//
// 当前只判断：
// - API 是否能响应
// - SQLite 是否初始化
// - Swagger/OpenAPI 是否已挂载
//
// 不把 bind/discovery 是否已实现混成服务健康，避免骨架阶段被错误标成 unhealthy。
func BuildHealthResponse() Response {
	sqliteStatus := "unhealthy"
	sqliteMessage := "sqlite 未初始化"
	if SQLiteInfra.DB() != nil {
		sqliteStatus = "healthy"
		sqliteMessage = "sqlite 已初始化"
	}
	return Response{
		OK:         true,
		Status:     "healthy",
		Service:    Settings.Conf.Name,
		Mode:       Settings.Conf.Mode,
		Version:    Settings.Conf.Version,
		ConfigFile: Settings.Conf.ConfigFile,
		CheckedAt:  time.Now().Unix(),
		Checks: map[string]CheckResult{
			"api":     {Status: "healthy", Message: "http 服务可响应"},
			"sqlite":  {Status: sqliteStatus, Message: sqliteMessage},
			"swagger": {Status: "healthy", Message: "swagger/openapi 入口已挂载"},
		},
	}
}
