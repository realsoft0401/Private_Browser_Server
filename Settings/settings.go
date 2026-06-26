package Settings

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var (
	// Conf 持有新的 Server 第一阶段全局配置。
	//
	// 设计来源：
	// - 这次重起仍然沿用 old / Client 的集中配置方式，避免业务层到处读环境变量；
	// - 第一阶段配置只保留最小必需域，不提前把 JWT、Env、Task、Dashboard 再拉回来；
	// - 后续如果扩配置，也应先补这里，再让业务层读取 `Settings.Conf`。
	Conf = new(AppConfig)

	configEngine = viper.New()
)

type AppConfig struct {
	Name        string `mapstructure:"name"`
	Mode        string `mapstructure:"mode"`
	Version     string `mapstructure:"version"`
	ProjectRoot string `mapstructure:"-"`
	ConfigFile  string `mapstructure:"-"`

	*ServerConfig     `mapstructure:"server"`
	*SQLiteConfig     `mapstructure:"sqlite"`
	*EdgeConfig       `mapstructure:"edge"`
	*DiscoveryConfig  `mapstructure:"discovery"`
	*NodeHealthConfig `mapstructure:"node_health"`
}

type ServerConfig struct {
	Host                string `mapstructure:"host"`
	Port                int    `mapstructure:"port"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds"`
}

type SQLiteConfig struct {
	Path         string `mapstructure:"path"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type EdgeConfig struct {
	RequestTimeoutSeconds int    `mapstructure:"request_timeout_seconds"`
	APIKey                string `mapstructure:"api_key"`
}

type DiscoveryConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	ListenAddress   string `mapstructure:"listen_address"`
	Port            int    `mapstructure:"port"`
	Magic           string `mapstructure:"magic"`
	ProtocolVersion int    `mapstructure:"protocol_version"`
	Group           string `mapstructure:"group"`
}

// NodeHealthConfig 控制中心节点在线状态的统一收口窗口。
//
// 设计来源：
// - 这次已经收口成 `healthy / offline` 两态，不再保留 `stale`；
// - 但“多久没 heartbeat 算 offline”仍然必须可配置，避免测试环境和正式环境窗口写死；
// - 因此这里只保留最小两项：巡检频率、离线阈值。
type NodeHealthConfig struct {
	MonitorIntervalSeconds int `mapstructure:"monitor_interval_seconds"`
	OfflineAfterSeconds    int `mapstructure:"offline_after_seconds"`
}

// Init 加载新的第一阶段配置文件。
//
// 当前只认 `config-docker.yaml`，因为这是你这次重起后的唯一施工口径。
func Init(projectRoot string) error {
	configFile := filepath.Join(projectRoot, "Settings", "config-docker.yaml")

	configEngine = viper.New()
	configEngine.SetConfigFile(configFile)
	configEngine.SetConfigType("yaml")
	setDefaults()

	if err := configEngine.ReadInConfig(); err != nil {
		return fmt.Errorf("read config failed: %w", err)
	}
	if err := configEngine.Unmarshal(Conf); err != nil {
		return fmt.Errorf("unmarshal config failed: %w", err)
	}
	normalizeConfig(projectRoot, configFile, Conf)

	configEngine.WatchConfig()
	configEngine.OnConfigChange(func(event fsnotify.Event) {
		updated := new(AppConfig)
		if err := configEngine.Unmarshal(updated); err != nil {
			fmt.Printf("reload config failed, err:%v\n", err)
			return
		}
		normalizeConfig(projectRoot, configFile, updated)
		Conf = updated
		fmt.Printf("config reloaded: %s\n", event.Name)
	})
	return nil
}

func setDefaults() {
	configEngine.SetDefault("name", "private-browser-server")
	configEngine.SetDefault("mode", "production")
	configEngine.SetDefault("version", "0.1.0-phase1")
	configEngine.SetDefault("server.host", "0.0.0.0")
	configEngine.SetDefault("server.port", 3400)
	configEngine.SetDefault("server.read_timeout_seconds", 15)
	configEngine.SetDefault("server.write_timeout_seconds", 30)
	configEngine.SetDefault("sqlite.path", "data/private_browser_server.db")
	configEngine.SetDefault("sqlite.max_open_conns", 1)
	configEngine.SetDefault("sqlite.max_idle_conns", 1)
	configEngine.SetDefault("edge.request_timeout_seconds", 20)
	configEngine.SetDefault("edge.api_key", "private-browser-edge-key")
	configEngine.SetDefault("discovery.enabled", true)
	configEngine.SetDefault("discovery.listen_address", "0.0.0.0")
	configEngine.SetDefault("discovery.port", 43000)
	configEngine.SetDefault("discovery.magic", "PRIVATE_BROWSER_CLIENT_DISCOVERY")
	configEngine.SetDefault("discovery.protocol_version", 1)
	configEngine.SetDefault("discovery.group", "default")
	configEngine.SetDefault("node_health.monitor_interval_seconds", 15)
	configEngine.SetDefault("node_health.offline_after_seconds", 90)
}

func normalizeConfig(projectRoot, configFile string, config *AppConfig) {
	config.ProjectRoot = projectRoot
	config.ConfigFile = configFile
	if config.ServerConfig == nil {
		config.ServerConfig = &ServerConfig{}
	}
	if config.SQLiteConfig == nil {
		config.SQLiteConfig = &SQLiteConfig{}
	}
	if config.EdgeConfig == nil {
		config.EdgeConfig = &EdgeConfig{}
	}
	if config.DiscoveryConfig == nil {
		config.DiscoveryConfig = &DiscoveryConfig{}
	}
	if config.NodeHealthConfig == nil {
		config.NodeHealthConfig = &NodeHealthConfig{}
	}

	if strings.TrimSpace(config.ServerConfig.Host) == "" {
		config.ServerConfig.Host = "0.0.0.0"
	}
	if config.ServerConfig.Port <= 0 {
		config.ServerConfig.Port = 3400
	}
	if config.ServerConfig.ReadTimeoutSeconds <= 0 {
		config.ServerConfig.ReadTimeoutSeconds = 15
	}
	if config.ServerConfig.WriteTimeoutSeconds <= 0 {
		config.ServerConfig.WriteTimeoutSeconds = 30
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
	if strings.TrimSpace(config.EdgeConfig.APIKey) == "" {
		config.EdgeConfig.APIKey = "private-browser-edge-key"
	}
	if strings.TrimSpace(config.DiscoveryConfig.ListenAddress) == "" {
		config.DiscoveryConfig.ListenAddress = "0.0.0.0"
	}
	if config.DiscoveryConfig.Port <= 0 {
		config.DiscoveryConfig.Port = 43000
	}
	if strings.TrimSpace(config.DiscoveryConfig.Magic) == "" {
		config.DiscoveryConfig.Magic = "PRIVATE_BROWSER_CLIENT_DISCOVERY"
	}
	if config.DiscoveryConfig.ProtocolVersion <= 0 {
		config.DiscoveryConfig.ProtocolVersion = 1
	}
	if strings.TrimSpace(config.DiscoveryConfig.Group) == "" {
		config.DiscoveryConfig.Group = "default"
	}
	if config.NodeHealthConfig.MonitorIntervalSeconds <= 0 {
		config.NodeHealthConfig.MonitorIntervalSeconds = 15
	}
	if config.NodeHealthConfig.OfflineAfterSeconds <= 0 {
		config.NodeHealthConfig.OfflineAfterSeconds = 90
	}
}
