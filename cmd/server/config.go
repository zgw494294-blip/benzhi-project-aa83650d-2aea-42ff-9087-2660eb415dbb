package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

type config struct {
	Addr      string
	DataDir   string
	SelfCheck bool
}

func parseConfig() (config, error) {
	addr := flag.String("addr", "", "监听地址，仅允许回环地址")
	data := flag.String("data", "./var/data", "本地事件账本目录")
	selfcheck := flag.Bool("selfcheck", false, "通过真实 HTTP 完成有界全流程自检")
	flag.Parse()
	resolved := strings.TrimSpace(*addr)
	if resolved == "" {
		if port := strings.TrimSpace(os.Getenv("PORT")); port != "" {
			value, err := strconv.Atoi(port)
			if err != nil || value < 1 || value > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1 到 65535 的端口号")
			}
			resolved = net.JoinHostPort("127.0.0.1", strconv.Itoa(value))
		} else {
			resolved = "127.0.0.1:19081"
		}
	}
	if err := validateAddress(resolved); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*data) == "" {
		return config{}, fmt.Errorf("-data 不能为空")
	}
	return config{Addr: resolved, DataDir: *data, SelfCheck: *selfcheck}, nil
}

func validateAddress(address string) error {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("-addr 格式无效：%w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("-addr 端口必须在 1 到 65535 之间")
	}
	if host == "localhost" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("-addr 必须绑定回环地址，拒绝 %q", host)
	}
	return nil
}
