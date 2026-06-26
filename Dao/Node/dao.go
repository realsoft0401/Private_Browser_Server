package Node

type Row struct {
	ClientID                string
	MainAccountID           string
	ClientSequence          int64
	Name                    string
	ClientIP                string
	BaseURL                 string
	DockerAPIURL            string
	OS                      string
	Arch                    string
	CPUCores                int64
	MemoryTotalMB           int64
	DockerVersion           string
	HealthStatus            string
	DiscoveryStatus         string
	DiscoveryReason         string
	PushStatus              string
	APIKeyHash              string
	LastDiscoveredAt        int64
	LastHeartbeatAt         int64
	LastHeartbeatReportedAt int64
	LastHeartbeatSource     string
	LastCheckedAt           int64
	LastError               string
	CreatedByUserID         string
	CreatedByUsername       string
	CreatedAt               int64
	UpdatedAt               int64
	DeletedAt               int64
}
