package ImagePolicy

// ImagePolicy 描述不同架构和渠道对应的浏览器镜像。
//
// 镜像选择必须由 Server 后端根据节点 arch 和策略决定，前端不能随意传镜像字符串。
type ImagePolicy struct {
	ID        string `json:"id"`
	Arch      string `json:"arch"`
	Channel   string `json:"channel"`
	Image     string `json:"image"`
	Tag       string `json:"tag"`
	Enabled   bool   `json:"enabled"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

const (
	ImageChannelStable = "stable"
	ImageChannelDev    = "dev"
)
