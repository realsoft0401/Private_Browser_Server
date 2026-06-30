package Quota

// RefreshRequest 描述一次平台额度快照刷新请求。
//
// 设计来源：
// - 平台正式接口还没接入前，Node 仍需要一个受控入口落额度快照，验证 run admission 链路；
// - 因此这里先允许管理员手工写入最新额度结果，后续平台接口到位后直接替换数据来源；
// - 这条请求只服务中心治理，不代表未来平台正式协议。
type RefreshRequest struct {
	QuotaLimit             int64  `json:"quotaLimit"`
	QuotaUsedSnapshot      int64  `json:"quotaUsedSnapshot"`
	QuotaAvailableSnapshot int64  `json:"quotaAvailableSnapshot"`
	ExpiresAt              int64  `json:"expiresAt"`
	Status                 string `json:"status"`
	LastError              string `json:"lastError"`
	Source                 string `json:"source"`
}

// AdmissionResult 是 run admission 的统一判断结果。
//
// 职责边界：
// - 它只表达“当前中心口径下，这个节点是否允许进入 run”；
// - 不替代未来 browser-env 自身配置校验，也不替代 Edge 运行时校验；
// - 后续无论是公开接口、内部服务还是任务编排，都应复用这一个判断结果结构。
type AdmissionResult struct {
	Allowed    bool     `json:"allowed"`
	Status     string   `json:"status"`
	Reasons    []string `json:"reasons"`
	Suggestion string   `json:"suggestion"`
	CheckedAt  int64    `json:"checkedAt"`
}

// RunQuotaResponse 返回额度快照和当前 run admission 判断。
type RunQuotaResponse struct {
	ClientID               string          `json:"clientId"`
	QuotaLimit             int64           `json:"quotaLimit"`
	QuotaUsedSnapshot      int64           `json:"quotaUsedSnapshot"`
	QuotaAvailableSnapshot int64           `json:"quotaAvailableSnapshot"`
	FetchedAt              int64           `json:"fetchedAt"`
	ExpiresAt              int64           `json:"expiresAt"`
	Status                 string          `json:"status"`
	LastError              string          `json:"lastError"`
	Admission              AdmissionResult `json:"admission"`
}
