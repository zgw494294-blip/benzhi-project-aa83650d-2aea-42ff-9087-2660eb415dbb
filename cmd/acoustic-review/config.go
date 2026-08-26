package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultAddr = "127.0.0.1:19081"

type config struct {
	Addr      string
	DataDir   string
	SelfCheck bool
}

func parseConfig(args []string) (config, error) {
	set := flag.NewFlagSet("acoustic-review", flag.ContinueOnError)
	set.SetOutput(os.Stderr)
	addr := set.String("addr", "", "监听地址，必须为回环地址")
	data := set.String("data", "./var/acoustic-review", "本地数据目录")
	selfcheck := set.Bool("selfcheck", false, "运行完整 HTTP 自检后退出")
	if err := set.Parse(args); err != nil {
		return config{}, err
	}
	if set.NArg() != 0 {
		return config{}, fmt.Errorf("不支持的位置参数")
	}
	resolved := strings.TrimSpace(*addr)
	if resolved == "" {
		resolved = defaultAddr
		if raw := strings.TrimSpace(os.Getenv("PORT")); raw != "" {
			port, err := strconv.Atoi(raw)
			if err != nil || port < 1 || port > 65535 {
				return config{}, fmt.Errorf("PORT 必须是 1-65535 的端口号")
			}
			resolved = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	host, portText, err := net.SplitHostPort(resolved)
	if err != nil {
		return config{}, fmt.Errorf("-addr 格式无效：%w", err)
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return config{}, fmt.Errorf("-addr 必须使用明确的回环 IP")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return config{}, fmt.Errorf("-addr 端口必须位于 1-65535")
	}
	if strings.TrimSpace(*data) == "" {
		return config{}, fmt.Errorf("-data 不能为空")
	}
	return config{Addr: resolved, DataDir: *data, SelfCheck: *selfcheck}, nil
}
