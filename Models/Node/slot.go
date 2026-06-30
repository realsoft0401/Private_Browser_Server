package Node

// EdgeClientSlot 是 Node Server 当前绑定关系下的 slot 明细视图。
//
// 设计来源：
// - 用户已经收口：slot 正式状态只有 waiting / loading / running / ending；
// - Node 需要一张中心子表缓存当前节点有哪些 slot、每个 slot 当前占用情况；
// - 但最终 slot 事实仍以 Client 上报和 slot_reconcile 结果为准，因此这里只表达中心缓存，不表达真实资产正文。
type EdgeClientSlot struct {
	ID            int64  `json:"id"`
	ClientID      string `json:"clientId"`
	SlotID        string `json:"slotId"`
	Status        string `json:"status"`
	CurrentEnvID  string `json:"currentEnvId"`
	CurrentRunID  string `json:"currentRunId"`
	ContainerID   string `json:"containerId"`
	ContainerName string `json:"containerName"`
	CDPPort       int64  `json:"cdpPort"`
	VNCPort       int64  `json:"vncPort"`
	LastError     string `json:"lastError"`
	LastSyncedAt  int64  `json:"lastSyncedAt"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}

// EdgeClientSlotLog 是 Node Server 的 slot 治理留痕。
//
// 职责边界：
// - 它只记录 create-slot / destroy-slot / reinit-slot / slot_reconcile 等治理动作摘要；
// - 不保存 SSE 全量事件，不替代 edge_client_slots 当前事实；
// - 后续管理员排障先看动作日志，再结合 slot 当前摘要和服务日志判断。
type EdgeClientSlotLog struct {
	ID               int64  `json:"id"`
	ClientID         string `json:"clientId"`
	SlotID           string `json:"slotId"`
	Action           string `json:"action"`
	Result           string `json:"result"`
	EnvID            string `json:"envId"`
	RunID            string `json:"runId"`
	Message          string `json:"message"`
	OperatorUserID   string `json:"operatorUserId"`
	OperatorUsername string `json:"operatorUsername"`
	CreatedAt        int64  `json:"createdAt"`
}
