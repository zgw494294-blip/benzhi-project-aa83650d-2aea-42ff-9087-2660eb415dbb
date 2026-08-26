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

	"acousticverdictworkbench/internal/application"
	"acousticverdictworkbench/internal/repository"
	"acousticverdictworkbench/internal/webui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("服务退出：%v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	cleanup := func() {}
	if cfg.SelfCheck {
		cfg.DataDir, err = os.MkdirTemp("", "acoustic-verdict-selfcheck-*")
		if err != nil {
			return fmt.Errorf("创建自检数据目录：%w", err)
		}
		cleanup = func() { _ = os.RemoveAll(cfg.DataDir) }
		defer cleanup()
	}
	repo, err := repository.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("恢复本地数据：%w", err)
	}
	defer repo.Close()
	service := application.New(repo)
	handler := webui.New(service).Handler()
	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("监听 %s：%w", cfg.Addr, err)
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	if cfg.SelfCheck {
		return selfCheckLifecycle(server, listener, serveErrors)
	}
	log.Printf("声学裁决工作台已监听 http://%s", listener.Addr())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case sig := <-signals:
		log.Printf("收到 %s，开始安全关闭", sig)
	case err := <-serveErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}

func selfCheckLifecycle(server *http.Server, listener net.Listener, serveErrors <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	checkErr := runSelfCheck(ctx, "http://"+listener.Addr().String())
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
	log.Printf("自检通过：建批、双标、仲裁、质量门禁、封存和写保护均已验证")
	return nil
}
