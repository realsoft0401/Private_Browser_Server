package main

import (
	"fmt"
	"os"
	"path/filepath"

	"private_browser_server/Infrastructures"
)

// main 是 Private_Browser_Server 的纯后端启动入口。
//
// 设计来源：
// - Server 是中心调度层，和 Private_Browser_Client 的边缘服务入口保持同样的“薄 main”结构；
// - main 只负责定位项目根目录和启动基础设施，不承载 Auth、Node、Env 等业务逻辑；
// - 后续新增 MySQL、任务队列、审计日志时，应继续挂到 Infrastructures.Init，不要让入口文件膨胀。
func main() {
	projectRoot, err := detectProjectRoot()
	if err != nil {
		fmt.Printf("detect project root failed, err:%v\n", err)
		os.Exit(1)
	}
	if err = Infrastructures.Init(projectRoot); err != nil {
		fmt.Printf("init infrastructure failed, err:%v\n", err)
		os.Exit(1)
	}
}

// detectProjectRoot 负责识别 Server 子项目根目录。
//
// 它沿用 Client 的根目录识别方式：优先从当前工作目录和可执行文件目录向上查找 Settings/config-dev.yaml。
// 这样本地开发、二进制运行和后续容器部署都能稳定找到配置、docs 等相对路径资源。
func detectProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	candidates := []string{cwd}
	if exePath, exeErr := os.Executable(); exeErr == nil {
		candidates = append(candidates, filepath.Dir(exePath))
	}

	for _, start := range candidates {
		current := start
		for {
			configPath := filepath.Join(current, "Settings", "config-dev.yaml")
			if _, statErr := os.Stat(configPath); statErr == nil {
				return current, nil
			}
			parent := filepath.Dir(current)
			if parent == current {
				break
			}
			current = parent
		}
	}

	return "", fmt.Errorf("project root not found")
}
