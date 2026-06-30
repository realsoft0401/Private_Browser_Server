package BrowserEnv

// ListQuery 是中心 browser-env 列表查询的最小过滤条件。
//
// 设计来源：
// - 当前中心层先以 `server_browser_envs` 为主视图，不直接把 Edge 查询接口透传上来；
// - 因此列表查询只围绕中心聚合表已有字段过滤，避免第一版就把分页、排序、全字段检索做散；
// - 目前最稳定的筛选维度是账号、节点、用户、RPA 类型和主状态。
type ListQuery struct {
	AccountID string
	ClientID  string
	UserID    string
	RPAType   string
	Status    string
}

// ListResponse 是中心 browser-env 列表查询结果。
type ListResponse struct {
	Items []ServerBrowserEnv `json:"items"`
	Total int                `json:"total"`
}
