package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"phonemereleasedesk/internal/application"
	"phonemereleasedesk/internal/persistence"
	webadapter "phonemereleasedesk/internal/web"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	repo, err := persistence.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("打开本地仓储：%w", err)
	}
	defer repo.Close()
	service := application.New(repo)
	handler := webadapter.New(service).Handler()
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s：%w", cfg.Addr, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	if cfg.SelfCheck {
		checkCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		checkErr := runSelfCheck(checkCtx, "http://"+listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		serveErr := <-serveErrors
		if checkErr != nil {
			return fmt.Errorf("自检失败：%w", checkErr)
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		log.Printf("自检通过：完整发布流程与凭据摘要核验成功")
		return nil
	}
	log.Printf("PhonemeReleaseDesk 已监听 %s", listener.Addr())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case signal := <-signals:
		log.Printf("收到 %s，开始安全关闭", signal)
	case serveErr := <-serveErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}
