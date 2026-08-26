package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type config struct {
	addr      string
	dataDir   string
	selfcheck bool
}

func parseConfig() (config, error) {
	defaultAddr := "127.0.0.1:19081"
	if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
		port, err := strconv.Atoi(raw)
		if err != nil || port < 1024 || port > 65535 {
			return config{}, fmt.Errorf("PORT 必须是 1024 至 65535 的端口号")
		}
		defaultAddr = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	}
	var cfg config
	flag.StringVar(&cfg.addr, "addr", defaultAddr, "回环监听地址")
	flag.StringVar(&cfg.dataDir, "data", filepath.Join("data", "voice-clarity"), "本地数据目录")
	flag.BoolVar(&cfg.selfcheck, "selfcheck", false, "执行真实 HTTP 正常流程自检后退出")
	flag.Parse()
	if err := validateAddress(cfg.addr); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.dataDir) == "" {
		return config{}, fmt.Errorf("数据目录不能为空")
	}
	return cfg, nil
}

func validateAddress(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("监听地址必须采用 host:port 格式: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1024 || port > 65535 {
		return fmt.Errorf("监听端口必须在 1024 至 65535 之间")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("监听地址必须是明确的回环地址")
	}
	return nil
}
