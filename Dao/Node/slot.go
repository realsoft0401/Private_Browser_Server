package Node

// SlotRow 是 edge_client_slots 的仓储写入结构。
//
// 这里单独抽出来，是为了让 slot_reconcile、rebind 后重建、后续节点治理动作都复用统一字段集合，
// 避免在多个 service 里各自拼 SQL 字段导致状态口径漂移。
type SlotRow struct {
	ID            int64
	ClientID      string
	SlotID        string
	Status        string
	CurrentEnvID  string
	CurrentRunID  string
	ContainerID   string
	ContainerName string
	CDPPort       int64
	VNCPort       int64
	LastError     string
	LastSyncedAt  int64
	CreatedAt     int64
	UpdatedAt     int64
}

// SlotLogRow 是 edge_client_slot_logs 的仓储写入结构。
type SlotLogRow struct {
	ID               int64
	ClientID         string
	SlotID           string
	Action           string
	Result           string
	EnvID            string
	RunID            string
	Message          string
	OperatorUserID   string
	OperatorUsername string
	CreatedAt        int64
}
