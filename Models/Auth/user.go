package Auth

// User 表示中心服务端的商业用户。
//
// Server 才负责用户体系；Client 不应恢复 users 表或 JWT 逻辑。
// PasswordHash 只能保存哈希，不能保存明文密码。
type User struct {
	ID           string `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

const (
	UserRoleAdmin = "admin"
	UserRoleUser  = "user"

	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)
