package ImagePolicy

// ImagePolicy 描述不同架构和渠道对应的浏览器镜像。
//
// 镜像选择必须由 Server 后端根据节点 arch 和策略决定，普通前端或用户不能随意传镜像字符串。
// 当前过渡阶段允许 Platform 受控下发一个“imagePolicy 值”，这个值暂时直接等于已登记镜像字符串；
// Node Server 仍要校验该值是否属于已登记策略，不能把任意镜像字符串都当作合法输入。
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
