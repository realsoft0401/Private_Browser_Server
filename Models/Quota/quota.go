package Quota

// ClientRunQuota 是平台额度在 Node 本地的可信快照。
//
// 职责边界：
// - Platform 仍然是真正额度来源；
// - Node 这里只缓存最近一次可信结果，给 run 准入和排障使用；
// - 因此不能把它误当成最终真相源，更不能脱离 Platform 长期自治。
type ClientRunQuota struct {
	ClientID               string `json:"clientId"`
	QuotaLimit             int64  `json:"quotaLimit"`
	QuotaUsedSnapshot      int64  `json:"quotaUsedSnapshot"`
	QuotaAvailableSnapshot int64  `json:"quotaAvailableSnapshot"`
	FetchedAt              int64  `json:"fetchedAt"`
	ExpiresAt              int64  `json:"expiresAt"`
	Status                 string `json:"status"`
	LastError              string `json:"lastError"`
}
