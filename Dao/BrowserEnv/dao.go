package BrowserEnv

type Row struct {
	EnvID           string
	MainAccountID   string
	ClientID        string
	UserID          string
	RPAType         string
	Name            string
	Status          string
	ContainerStatus string
	RuntimeStatus   string
	CurrentSlotID   string
	CDPURL          string
	WebVNCURL       string
	LastTaskID      string
	LastError       string
	LastSyncedAt    int64
	CreatedAt       int64
	UpdatedAt       int64
	DeletedAt       int64
}
