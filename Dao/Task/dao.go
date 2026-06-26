package Task

type Row struct {
	ID               string
	MainAccountID    string
	OperatorUserID   string
	OperatorUsername string
	ClientID         string
	EnvID            string
	TaskType         string
	ResourceType     string
	ResourceID       string
	Status           string
	EdgeTaskID       string
	EventsURL        string
	ErrorMessage     string
	Suggestion       string
	CreatedAt        int64
	UpdatedAt        int64
	FinishedAt       int64
}
