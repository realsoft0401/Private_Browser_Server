package Quota

type Row struct {
	ClientID               string
	QuotaLimit             int64
	QuotaUsedSnapshot      int64
	QuotaAvailableSnapshot int64
	FetchedAt              int64
	ExpiresAt              int64
	Status                 string
	LastError              string
}
