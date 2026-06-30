package BrowserEnv

// DeleteBrowserEnvImageResult 是中心 `/del` 返回的一条镜像删除明细。
//
// 设计来源：
// - Client `/del` 已经把结果收口成 `image + deleted + untagged` 三字段；
// - Node Server 不应重新发明第二套删除结果模型，否则中心与边缘文档会漂移；
// - 因此这里直接镜像正式边缘返回结构，只保留中心调用方真正需要的字段。
type DeleteBrowserEnvImageResult struct {
	Image    string `json:"image"`
	Deleted  string `json:"deleted,omitempty"`
	Untagged string `json:"untagged,omitempty"`
}

// DeleteBrowserEnvImageResponse 是中心 browser-env `/del` 接口的同步结果。
//
// 职责边界：
// - 这条结果只表达镜像删除事实，不表达环境包资产是否被删除；
// - 中心不会因为 `/del` 成功而删掉 `server_browser_envs` 主记录；
// - 调用方仍然应该把这条接口理解成“镜像清理动作”，不是“环境包退场动作”。
type DeleteBrowserEnvImageResponse struct {
	EnvID          string                        `json:"envId"`
	Image          string                        `json:"image"`
	ImageRemoved   bool                          `json:"imageRemoved"`
	Results        []DeleteBrowserEnvImageResult `json:"results"`
	WarningMessage string                        `json:"warningMessage"`
	DeletedAt      int64                         `json:"deletedAt"`
}
