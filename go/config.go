package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config 是应用的全部可调参数，一律来自环境变量。
// 同时兼容新前缀 FNPORTGATE_* 与历史前缀 FN_UPNP_*，便于平滑升级。
type Config struct {
	WebHost          string
	WebPort          int
	DataDir          string
	SocketPath       string
	GatewayPrefix    string
	DiscoveryTimeout time.Duration
	DiscoveryTTL     time.Duration
	LogLevel         string
}

func envString(names []string, def string) string {
	for _, name := range names {
		if raw, ok := os.LookupEnv(name); ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}
	return def
}

func envInt(names []string, def int) (int, error) {
	raw := envString(names, "")
	if raw == "" {
		return def, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("环境变量 %s 必须是整数，当前值: %q", names[0], raw)
	}
	return value, nil
}

func loadConfig() (*Config, error) {
	port, err := envInt([]string{"FNPORTGATE_WEB_PORT", "FN_UPNP_WEB_PORT"}, 8090)
	if err != nil {
		return nil, err
	}
	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("服务端口超出范围（1-65535）：%d", port)
	}

	discTimeout, err := envInt([]string{"FNPORTGATE_DISCOVERY_TIMEOUT", "FN_UPNP_DISCOVERY_TIMEOUT"}, 5)
	if err != nil {
		return nil, err
	}
	if discTimeout < 1 || discTimeout > 30 {
		return nil, fmt.Errorf("发现超时超出范围（1-30 秒）：%d", discTimeout)
	}

	ttl, err := envInt([]string{"FNPORTGATE_DISCOVERY_TTL", "FN_UPNP_DISCOVERY_TTL"}, 300)
	if err != nil {
		return nil, err
	}
	if ttl < 5 {
		ttl = 5
	}

	dataDir := envString([]string{"FNPORTGATE_DATA_DIR", "FN_UPNP_DATA_DIR"}, "/data")
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, fmt.Errorf("数据目录无效：%w", err)
	}

	return &Config{
		WebHost:       envString([]string{"FNPORTGATE_WEB_HOST", "FN_UPNP_WEB_HOST"}, "0.0.0.0"),
		WebPort:       port,
		DataDir:       absDataDir,
		SocketPath:    envString([]string{"FNPORTGATE_SOCKET"}, ""),
		GatewayPrefix: strings.TrimSuffix(envString([]string{"FNPORTGATE_GATEWAY_PREFIX"}, "/app/fnportgate"), "/"),

		DiscoveryTimeout: time.Duration(discTimeout) * time.Second,
		DiscoveryTTL:     time.Duration(ttl) * time.Second,
		LogLevel:         strings.ToUpper(envString([]string{"FNPORTGATE_LOG_LEVEL", "FN_UPNP_LOG_LEVEL"}, "INFO")),
	}, nil
}

// configPath 返回端口映射持久化文件路径。
func (c *Config) configPath() string {
	return filepath.Join(c.DataDir, "fnportgate.json")
}

// listenAddr 返回 HTTP 监听地址。
func (c *Config) listenAddr() string {
	return fmt.Sprintf("%s:%d", c.WebHost, c.WebPort)
}
