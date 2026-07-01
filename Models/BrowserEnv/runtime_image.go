package BrowserEnv

// UpdateRuntimeImageRequest 是中心修改 browser-env 正式运行镜像的请求体。
//
// 设计边界：
// - 这里只允许传完整 Docker image 引用；
// - 不允许携带 slotId、force、proxy、fingerprint 或 Docker HostConfig；
// - 镜像是否已存在、是否需要拉取，必须走独立 pull-image/run 链路确认。
type UpdateRuntimeImageRequest struct {
	Image string `json:"image"`
}

// UpdateRuntimeImageResponse 是中心同步返回的 runtime.image 修改结果。
type UpdateRuntimeImageResponse struct {
	EnvID         string `json:"envId"`
	Status        string `json:"status"`
	PreviousImage string `json:"previousImage"`
	Image         string `json:"image"`
	UpdatedAt     int64  `json:"updatedAt"`
}
