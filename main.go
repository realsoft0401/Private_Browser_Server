package main

import (
	"fmt"
	"os"
	"path/filepath"

	"private_browser_server/Infrastructures"
)

// main 是新的 Private_Browser_Server 第一阶段启动入口。
//
// 设计来源：
//   - 这次 Server 重起已经明确只先做 bind 主线，不把旧项目所有业务一起搬回来；
//   - 因此入口必须继续保持“薄 main”，只负责找根目录和启动基础设施；
//   - 后续如果加 discovery listener、bind service、push-client-id，也应继续挂在基础设施和路由层，
//     不要把业务判断重新塞回入口。
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

// detectProjectRoot 负责识别新的 Server 子项目根目录。
//
// 这里继续沿用 Client 和 old Server 的做法：
// - 优先从当前目录和可执行文件目录向上找配置文件；
// - 这样本地 `go run .`、二进制运行、容器运行都能稳定拿到 Settings/docs/public 路径；
// - 不要把相对路径判断复制到各个 Service 里。
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
			configPath := filepath.Join(current, "Settings", "config-docker.yaml")
			if stat, statErr := os.Stat(configPath); statErr == nil && !stat.IsDir() {
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
