package Bind

type LogRow struct {
	ID               int64
	ClientID         string
	MainAccountID    string
	ClientIP         string
	OperatorUserID   string
	OperatorUsername string
	Action           string
	Result           string
	Message          string
	CreatedAt        int64
}
