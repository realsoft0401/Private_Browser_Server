package Routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"private_browser_server/Settings"
)

func TestDocumentationEntrypoints(t *testing.T) {
	Settings.Conf.ProjectRoot = detectTestProjectRoot(t)

	router := Setup()

	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{name: "swagger", path: "/swagger", contains: "swagger-ui"},
		{name: "scalar", path: "/scalar", contains: "@scalar/api-reference"},
		{name: "admin", path: "/admin", contains: "Node Admin Demo"},
		{name: "openapi", path: "/openapi.yaml", contains: "openapi: 3.0.3"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, tc.path, nil)
			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("unexpected status code: got=%d want=%d", recorder.Code, http.StatusOK)
			}
			if !strings.Contains(recorder.Body.String(), tc.contains) {
				t.Fatalf("response for %s missing marker %q", tc.path, tc.contains)
			}
		})
	}
}

// detectTestProjectRoot 为 Routes 层文档入口测试定位 Server 项目根目录。
//
// 设计来源：
// - 这些测试只验证静态文档路由是否真实挂载，不需要初始化 SQLite、UDP discovery 或完整基础设施；
// - Routes.Setup 读取 `Settings.Conf.ProjectRoot` 来返回 `docs/openapi.yaml` 和 `public/*.html`；
// - 因此测试只需要从当前工作目录向上找到带 `go.mod`、`docs/openapi.yaml`、`public/swagger.html` 的目录。
func detectTestProjectRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working dir failed: %v", err)
	}
	for {
		if fileExists(filepath.Join(dir, "go.mod")) &&
			fileExists(filepath.Join(dir, "docs", "openapi.yaml")) &&
			fileExists(filepath.Join(dir, "public", "swagger.html")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("project root not found from %s", dir)
		}
		dir = parent
	}
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}
