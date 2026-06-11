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

	*ServerConfig    `mapstructure:"server"`
	*SQLiteConfig    `mapstructure:"sqlite"`
	*JWTConfig       `mapstructure:"jwt"`
	*EdgeConfig      `mapstructure:"edge"`
	*TaskConfig      `mapstructure:"task"`
	*DiscoveryConfig `mapstructure:"discovery"`
}

// ServerConfig 描述中心服务自身的监听参数。
//
// 它只表示 Private_Browser_Server 的 HTTP 入口，不表示 Edge Client 地址。
type ServerConfig struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
}

// SQLiteConfig 描述 Node Server 的本地中心数据库。
//
// 这是用户确认后的 Node Server 口径：平台管理端使用 MySQL，
// Node Server 只保存本节点控制面需要的节点、环境聚合和任务摘要，因此用 SQLite 降低部署成本。
// 后续 Repository 只能通过 Rom.DB() 使用连接，不能在业务层重新打开数据库。
type SQLiteConfig struct {
	Path         string `mapstructure:"path"`
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

// EdgeConfig 描述 Server 调用 Edge Client 的通用 HTTP 策略。
//
// 单个 Edge Client 的 baseUrl/apiKey 由 edge_clients 保存；这里不保存 Client 列表。
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

// DiscoveryConfig 描述 Node Server 监听 Client UDP beacon 的策略。
//
// 设计来源：Client 会在独立内网广播本机 Edge 服务入口；Server 监听后只能把它当“发现线索”，
// 仍必须再做 /health、/api/v1/edge/device-info、Docker 2375 和架构归一化，不能直接进入 verified。
type DiscoveryConfig struct {
	Enabled             bool   `mapstructure:"enabled"`
	ListenAddress       string `mapstructure:"listen_address"`
	Port                int    `mapstructure:"port"`
	Magic               string `mapstructure:"magic"`
	ProtocolVersion     int    `mapstructure:"protocol_version"`
	Group               string `mapstructure:"group"`
	MaxPacketBytes      int    `mapstructure:"max_packet_bytes"`
	StaleAfterSeconds   int    `mapstructure:"stale_after_seconds"`
	OfflineAfterSeconds int    `mapstructure:"offline_after_seconds"`
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
	configEngine.SetDefault("server.port", 3400)
	configEngine.SetDefault("server.read_timeout_seconds", 15)
	// Node Server 的 run 现在可能在同一请求里同步等待 Edge pull-image 终态，
	// 因此写超时不能继续维持 15 秒，否则会出现任务已经落库 failed，但 HTTP 响应先被服务端切断的假象。
	configEngine.SetDefault("server.write_timeout_seconds", 720)
	configEngine.SetDefault("sqlite.path", "data/private_browser_server.db")
	configEngine.SetDefault("sqlite.max_open_conns", 1)
	configEngine.SetDefault("sqlite.max_idle_conns", 1)
	configEngine.SetDefault("jwt.secret", "dev-only-change-me")
	configEngine.SetDefault("jwt.expire_hours", 24)
	configEngine.SetDefault("jwt.issuer", "private-browser-server")
	configEngine.SetDefault("jwt.allow_weak_secret", env != "prod")
	configEngine.SetDefault("edge.request_timeout_seconds", 20)
	configEngine.SetDefault("edge.retry_times", 0)
	configEngine.SetDefault("task.refresh_interval_seconds", 5)
	configEngine.SetDefault("task.stale_seconds", 60)
	configEngine.SetDefault("discovery.enabled", true)
	configEngine.SetDefault("discovery.listen_address", "0.0.0.0")
	configEngine.SetDefault("discovery.port", 43000)
	configEngine.SetDefault("discovery.magic", "PRIVATE_BROWSER_CLIENT_DISCOVERY")
	configEngine.SetDefault("discovery.protocol_version", 1)
	configEngine.SetDefault("discovery.group", "default")
	configEngine.SetDefault("discovery.max_packet_bytes", 8192)
	configEngine.SetDefault("discovery.stale_after_seconds", 30)
	configEngine.SetDefault("discovery.offline_after_seconds", 90)
}

// normalizeConfig 补齐指针配置并收敛不合理值。
func normalizeConfig(projectRoot, configFile, env string, config *AppConfig) {
	config.ProjectRoot = projectRoot
	config.ConfigFile = configFile
	config.Env = env
	if config.ServerConfig == nil {
		config.ServerConfig = &ServerConfig{}
	}
	if config.SQLiteConfig == nil {
		config.SQLiteConfig = &SQLiteConfig{}
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
	if config.DiscoveryConfig == nil {
		config.DiscoveryConfig = &DiscoveryConfig{}
	}
	if strings.TrimSpace(config.SQLiteConfig.Path) == "" {
		config.SQLiteConfig.Path = "data/private_browser_server.db"
	}
	if !filepath.IsAbs(config.SQLiteConfig.Path) {
		config.SQLiteConfig.Path = filepath.Join(projectRoot, config.SQLiteConfig.Path)
	}
	if config.SQLiteConfig.MaxOpenConns <= 0 {
		config.SQLiteConfig.MaxOpenConns = 1
	}
	if config.SQLiteConfig.MaxIdleConns < 0 {
		config.SQLiteConfig.MaxIdleConns = 0
	}
	if config.EdgeConfig.RequestTimeoutSeconds <= 0 {
		config.EdgeConfig.RequestTimeoutSeconds = 20
	}
	if config.ServerConfig.WriteTimeoutSeconds <= 0 {
		config.ServerConfig.WriteTimeoutSeconds = 720
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
	normalizeDiscoveryConfig(config.DiscoveryConfig)
}

func normalizeDiscoveryConfig(config *DiscoveryConfig) {
	if config == nil {
		return
	}
	if strings.TrimSpace(config.ListenAddress) == "" {
		config.ListenAddress = "0.0.0.0"
	}
	if config.Port <= 0 {
		config.Port = 43000
	}
	if strings.TrimSpace(config.Magic) == "" {
		config.Magic = "PRIVATE_BROWSER_CLIENT_DISCOVERY"
	}
	if config.ProtocolVersion <= 0 {
		config.ProtocolVersion = 1
	}
	if strings.TrimSpace(config.Group) == "" {
		config.Group = "default"
	}
	if config.MaxPacketBytes <= 0 || config.MaxPacketBytes > 65535 {
		config.MaxPacketBytes = 8192
	}
	if config.StaleAfterSeconds <= 0 {
		config.StaleAfterSeconds = 30
	}
	if config.OfflineAfterSeconds <= 0 {
		config.OfflineAfterSeconds = 90
	}
	if config.OfflineAfterSeconds < config.StaleAfterSeconds {
		config.OfflineAfterSeconds = config.StaleAfterSeconds * 3
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
