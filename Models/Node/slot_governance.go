package Node

// SetTargetSlotCountRequest 描述一次管理员手工更新目标 slot 数的请求。
//
// 设计来源：
// - 平台正式下发链路还没接入前，Node 仍需要一个受控入口把 `target_slot_count` 落到中心库；
// - 否则 `slot_reconcile` 只能读到实际 slot 数，无法进入“目标数 vs 实际数”的治理阶段；
// - 这个入口当前只服务内网管理员治理，不代表未来平台正式协议。
type SetTargetSlotCountRequest struct {
	TargetSlotCount int64  `json:"targetSlotCount"`
	Source          string `json:"source"`
}

// SetTargetSlotCountResponse 返回一次目标 slot 数更新后的中心摘要。
type SetTargetSlotCountResponse struct {
	ClientID            string `json:"clientId"`
	TargetSlotCount     int64  `json:"targetSlotCount"`
	ActualSlotCount     int64  `json:"actualSlotCount"`
	AvailableSlotCount  int64  `json:"availableSlotCount"`
	RunningSlotCount    int64  `json:"runningSlotCount"`
	SlotExceptionStatus string `json:"slotExceptionStatus"`
	SlotExceptionReason string `json:"slotExceptionReason"`
	UpdatedAt           int64  `json:"updatedAt"`
}

// EdgeClientSlotListResponse 是中心节点 slot 明细查询结果。
//
// 职责边界：
// - 这里只返回 Node 已缓存的当前 slot 明细和主节点汇总；
// - 不直接返回 Client `/api/v1/edge/slots` 原始报文；
// - 真正 slot 真相仍以 Client 为准，这里是中心缓存视图。
type EdgeClientSlotListResponse struct {
	ClientID            string           `json:"clientId"`
	TargetSlotCount     int64            `json:"targetSlotCount"`
	ActualSlotCount     int64            `json:"actualSlotCount"`
	AvailableSlotCount  int64            `json:"availableSlotCount"`
	RunningSlotCount    int64            `json:"runningSlotCount"`
	SlotExceptionStatus string           `json:"slotExceptionStatus"`
	SlotExceptionReason string           `json:"slotExceptionReason"`
	LastSlotCheckedAt   int64            `json:"lastSlotCheckedAt"`
	Items               []EdgeClientSlot `json:"items"`
	Total               int              `json:"total"`
}
