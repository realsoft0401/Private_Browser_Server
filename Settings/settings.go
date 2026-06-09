package Settings

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	// BuildEnv 用于编译期注入默认运行环境，例如：
	// go build -ldflags "-X private_browser_server/Settings.BuildEnv=prod"
	BuildEnv = ""

	// Conf 是 Server 进程的全局配置对象。
	//
	// 当前项目沿用 Client 的集中配置模式，避免业务层到处读取环境变量或硬编码端口。
	Conf = new(AppConfig)

	configEngine = viper.New()
)

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Mode        string `mapstructure:"mode"`
	Version     string `mapstructure:"version"`
	ProjectRoot string `mapstructure:"-"`
	ConfigFile  string `mapstructure:"-"`
	Env         string `mapstructure:"-"`

	*ServerConfig `mapstructure:"server"`
	*MySQLConfig  `mapstructure:"mysql"`
	*JWTConfig    `mapstructure:"jwt"`
	*EdgeConfig   `mapstructure:"edge"`
	*TaskConfig   `mapstructure:"task"`
}

// ServerConfig 描述中心服务自身的监听参数。
//
// 它只表示 Private_Browser_Server 的 HTTP 入口，不表示 Edge 节点地址。
type ServerConfig struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
}

// MySQLConfig 描述中心数据库连接参数。
//
// V1 的用户、节点、环境聚合和任务状态都应落 MySQL；这里先定义配置契约，
// 下一步接入 GORM/MySQL 时不得让各业务层直接拼 DSN。
type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Database     string `mapstructure:"database"`
	Username     string `mapstructure:"username"`
	Password     string `mapstructure:"password"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

// JWTConfig 描述商业用户认证 Token 策略。
//
// Server 才负责 JWT；不要把这些配置同步回 Client。
type JWTConfig struct {
	Secret          string `mapstructure:"secret"`
	ExpireHours     int    `mapstructure:"expire_hours"`
	Issuer          string `mapstructure:"issuer"`
	AllowWeakSecret bool   `mapstructure:"allow_weak_secret"`
}

// EdgeConfig 描述 Server 调用 Edge 节点的通用 HTTP 策略。
//
// 单个节点的 baseUrl/apiKey 由 control_nodes 保存；这里不保存节点列表。
type EdgeConfig struct {
	RequestTimeoutSeconds int `mapstructure:"request_timeout_seconds"`
	// RetryTimes 是早期配置遗留字段，当前已废弃且 EdgeClient 不读取。
	//
	// 用户确认资产动作失败不能自动重试；保留字段只为旧配置文件解析不报错，后续迁移时可删除。
	RetryTimes int `mapstructure:"retry_times"`
}

// TaskConfig 描述 Server 任务状态刷新节奏。
//
// 它只定义中心任务观察周期，不直接控制 Edge 内部任务 worker。
type TaskConfig struct {
	RefreshIntervalSeconds int `mapstructure:"refresh_interval_seconds"`
	StaleSeconds           int `mapstructure:"stale_seconds"`
}

// Init 加载当前环境配置文件。
func Init(projectRoot string) error {
	env := resolveEnv()
	configFile := filepath.Join(projectRoot, "Settings", fmt.Sprintf("config-%s.yaml", env))

	configEngine = viper.New()
	configEngine.SetConfigFile(configFile)
	configEngine.SetConfigType("yaml")
	setDefaults(env)

	if err := configEngine.ReadInConfig(); err != nil {
		return fmt.Errorf("read config failed: %w", err)
	}
	if err := configEngine.Unmarshal(Conf); err != nil {
		return fmt.Errorf("unmarshal config failed: %w", err)
	}
	normalizeConfig(projectRoot, configFile, env, Conf)

	configEngine.WatchConfig()
	configEngine.OnConfigChange(func(event fsnotify.Event) {
		updated := new(AppConfig)
		if err := configEngine.Unmarshal(updated); err != nil {
			fmt.Printf("reload config failed, err:%v\n", err)
			return
		}
		normalizeConfig(projectRoot, configFile, env, updated)
		Conf = updated
		fmt.Printf("config reloaded: %s\n", event.Name)
	})
	return nil
}

// setDefaults 保持 Server 默认配置集中可查。
func setDefaults(env string) {
	configEngine.SetDefault("name", "private-browser-server")
	configEngine.SetDefault("mode", env)
	configEngine.SetDefault("version", "0.1.0")
	configEngine.SetDefault("server.host", "0.0.0.0")
	configEngine.SetDefault("server.port", 8080)
	configEngine.SetDefault("server.read_timeout_seconds", 15)
	configEngine.SetDefault("server.write_timeout_seconds", 15)
	configEngine.SetDefault("mysql.host", "127.0.0.1")
	configEngine.SetDefault("mysql.port", 3306)
	configEngine.SetDefault("mysql.database", "private_browser_server")
	configEngine.SetDefault("mysql.username", "root")
	configEngine.SetDefault("mysql.password", "")
	configEngine.SetDefault("mysql.max_open_conns", 20)
	configEngine.SetDefault("mysql.max_idle_conns", 5)
	configEngine.SetDefault("jwt.secret", "dev-only-change-me")
	configEngine.SetDefault("jwt.expire_hours", 24)
	configEngine.SetDefault("jwt.issuer", "private-browser-server")
	configEngine.SetDefault("jwt.allow_weak_secret", env != "prod")
	configEngine.SetDefault("edge.request_timeout_seconds", 20)
	configEngine.SetDefault("edge.retry_times", 0)
	configEngine.SetDefault("task.refresh_interval_seconds", 5)
	configEngine.SetDefault("task.stale_seconds", 60)
}

// normalizeConfig 补齐指针配置并收敛不合理值。
func normalizeConfig(projectRoot, configFile, env string, config *AppConfig) {
	config.ProjectRoot = projectRoot
	config.ConfigFile = configFile
	config.Env = env
	if config.ServerConfig == nil {
		config.ServerConfig = &ServerConfig{}
	}
	if config.MySQLConfig == nil {
		config.MySQLConfig = &MySQLConfig{}
	}
	if config.JWTConfig == nil {
		config.JWTConfig = &JWTConfig{}
	}
	if config.EdgeConfig == nil {
		config.EdgeConfig = &EdgeConfig{}
	}
	if config.TaskConfig == nil {
		config.TaskConfig = &TaskConfig{}
	}
	if config.MySQLConfig.Port <= 0 {
		config.MySQLConfig.Port = 3306
	}
	if config.EdgeConfig.RequestTimeoutSeconds <= 0 {
		config.EdgeConfig.RequestTimeoutSeconds = 20
	}
	if config.EdgeConfig.RetryTimes < 0 {
		config.EdgeConfig.RetryTimes = 0
	}
	if config.TaskConfig.RefreshIntervalSeconds <= 0 {
		config.TaskConfig.RefreshIntervalSeconds = 5
	}
	if config.TaskConfig.StaleSeconds < config.TaskConfig.RefreshIntervalSeconds*2 {
		config.TaskConfig.StaleSeconds = config.TaskConfig.RefreshIntervalSeconds * 2
	}
}

// resolveEnv 统一决定当前运行环境。
func resolveEnv() string {
	env := strings.TrimSpace(os.Getenv("ENV"))
	if env == "" {
		env = strings.TrimSpace(BuildEnv)
	}
	if env == "" {
		env = "dev"
	}
	return strings.ToLower(env)
}
