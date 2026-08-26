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
	"voice-clarity-acceptance/internal/store"
	"voice-clarity-acceptance/internal/web"
	"voice-clarity-acceptance/internal/workflow"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	dataDir := cfg.dataDir
	if cfg.selfcheck {
		dataDir, err = os.MkdirTemp("", "voice-clarity-selfcheck-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(dataDir)
	}
	repository, err := store.Open(dataDir)
	if err != nil {
		return fmt.Errorf("恢复本地存储: %w", err)
	}
	service := workflow.New(repository)
	server := &http.Server{
		Addr:              cfg.addr,
		Handler:           web.New(service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Serve(listener)
	}()
	if cfg.selfcheck {
		checkErr := runSelfcheck("http://" + listener.Addr().String())
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(ctx)
		serveErr := <-errCh
		if checkErr != nil {
			return checkErr
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		log.Printf("自检通过：建案、冻结计划、采集判定、审核、凭据校验和审计查询均完成")
		return nil
	}
	log.Printf("语音可懂度验收工作台已监听 http://%s", listener.Addr().String())
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-sigCtx.Done():
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
