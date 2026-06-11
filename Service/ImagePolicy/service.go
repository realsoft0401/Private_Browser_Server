package ImagePolicy

import (
	"fmt"
	"strings"

	imagePolicyModel "private_browser_server/Models/ImagePolicy"
	nodeModel "private_browser_server/Models/Node"
)

const (
	// defaultStableAMD64Image 来源于当前仓库根 README 已验证通过的 AMD64 商用镜像。
	//
	// 这里先用代码内最小默认值把 Node Server V1 create env 跑通；
	// 后续如果客户需要多版本、多渠道或灰度策略，再把这组默认值提升为 SQLite/config 管理。
	defaultStableAMD64Image = "crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.1-amd64"
	// defaultStableARM64Image 来源于当前仓库根 README 已验证通过的 ARM64 商用镜像。
	defaultStableARM64Image = "crpi-6s60spbjvluac8j8.cn-shanghai.personal.cr.aliyuncs.com/ln0216/private_browser_edge:1.1-arm64"
)

// ResolveRuntimeImage 根据中心 imagePolicy 和节点架构选择真正下发给 Edge 的 runtime.image。
//
// 设计来源：
//   - 用户明确要求浏览器镜像选择归 Node Server，不让前端直接决定商业镜像字符串；
//   - Client 只保存并执行环境包内的 runtime.image，不根据本机架构自行替换 tag；
//   - 当前 V1 先用稳定默认策略跑通 create env，后续再扩成持久化 ImagePolicy 子系统；
//   - 2026-06-10 起，Platform 会逐步直接下发一个受控 imagePolicy 值；过渡期该值先等于已登记镜像字符串，
//     例如 amd64 默认值就是 defaultStableAMD64Image，但 Node Server 仍要校验它是否属于受控策略。
//
// 职责边界：
// - 只做“imagePolicy + arch -> image”解析；
// - 不探测节点健康，不拉镜像，不写数据库；
// - arch=unknown 时必须失败，禁止偷偷兜底到任意镜像；
// - 即使 imagePolicy 现在看起来像镜像字符串，也必须命中已登记值，不能接受任意用户输入。
func ResolveRuntimeImage(policyValue string, arch string) (string, error) {
	arch = strings.TrimSpace(arch)
	defaultImage, err := defaultImageForArch(arch)
	if err != nil {
		return "", err
	}

	normalizedPolicy := strings.TrimSpace(policyValue)
	if normalizedPolicy == "" || strings.EqualFold(normalizedPolicy, imagePolicyModel.ImageChannelStable) {
		return defaultImage, nil
	}
	if strings.EqualFold(normalizedPolicy, imagePolicyModel.ImageChannelDev) {
		return "", fmt.Errorf("当前 Node Server V1 只配置了 stable 镜像策略，dev 渠道尚未启用")
	}

	switch normalizedPolicy {
	case defaultStableAMD64Image:
		if arch != nodeModel.NodeArchAMD64 {
			return "", fmt.Errorf("imagePolicy=%s 只允许 verified amd64 节点使用，当前节点架构=%s", normalizedPolicy, arch)
		}
		return defaultStableAMD64Image, nil
	case defaultStableARM64Image:
		if arch != nodeModel.NodeArchARM64 {
			return "", fmt.Errorf("imagePolicy=%s 只允许 verified arm64 节点使用，当前节点架构=%s", normalizedPolicy, arch)
		}
		return defaultStableARM64Image, nil
	default:
		return "", fmt.Errorf("不支持的 imagePolicy: %s；当前只接受 Platform 下发的已登记镜像值或兼容别名 stable", normalizedPolicy)
	}
}

func defaultImageForArch(arch string) (string, error) {
	switch arch {
	case nodeModel.NodeArchAMD64:
		return defaultStableAMD64Image, nil
	case nodeModel.NodeArchARM64:
		return defaultStableARM64Image, nil
	default:
		return "", fmt.Errorf("节点架构 %s 不可用于镜像策略；请先完成 verify 和架构归一化", arch)
	}
}
