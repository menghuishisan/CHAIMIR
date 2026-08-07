// provisioner main 启动独立的动态命名空间最小 RBAC 预配器。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chaimir/internal/platform/config"
	platformk8s "chaimir/internal/platform/k8s"
	"chaimir/pkg/logging"
)

// main 只负责装配预配器并把启动/运行错误显式返回到容器退出码。
func main() {
	if err := run(); err != nil {
		slog.Error("sandbox RBAC provisioner exited", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

// run 加载最小配置,建立集群客户端并运行命名空间控制循环。
func run() error {
	cfg, err := config.LoadSandboxRBACProvisioner()
	if err != nil {
		return err
	}
	logging.Setup(cfg.LogLevel, cfg.LogFormat)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	client, err := platformk8s.New(config.SandboxConfig{KubeconfigPath: cfg.KubeconfigPath})
	if err != nil {
		return err
	}
	provisioner, err := platformk8s.NewSandboxRBACProvisioner(client.Clientset(), cfg.ServiceAccountNamespace, cfg.ServiceAccountName, time.Duration(cfg.PollIntervalSeconds)*time.Second)
	if err != nil {
		return err
	}
	slog.Info("sandbox RBAC provisioner started", slog.String("service_account", cfg.ServiceAccountNamespace+"/"+cfg.ServiceAccountName))
	return provisioner.Run(ctx)
}
