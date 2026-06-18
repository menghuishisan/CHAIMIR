// cron main 是部署层受控定时任务命令入口,当前负责执行真实备份并记录到 M9。
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"chaimir/internal/modules/admin"
	"chaimir/internal/platform/config"
	"chaimir/internal/platform/db"
	"chaimir/internal/platform/storage"
	"chaimir/internal/platform/timex"
	"chaimir/pkg/logging"
	"chaimir/pkg/snowflake"
)

const backupContentType = "application/octet-stream"

// backupManifest 描述一次完整备份批次,记录 DB dump 与对象桶镜像的可恢复入口。
type backupManifest struct {
	Version        int                    `json:"version"`
	Batch          string                 `json:"batch"`
	CreatedAt      string                 `json:"created_at"`
	Database       backupManifestObject   `json:"database"`
	ObjectBuckets  []backupManifestBucket `json:"object_buckets"`
	TotalSizeBytes int64                  `json:"total_size_bytes"`
}

// backupManifestObject 描述清单中的单个备份对象。
type backupManifestObject struct {
	Bucket      string `json:"bucket"`
	Key         string `json:"key"`
	ObjectRef   string `json:"object_ref"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

// backupManifestBucket 描述一个源桶被复制到备份桶后的摘要。
type backupManifestBucket struct {
	SourceBucket string `json:"source_bucket"`
	DestPrefix   string `json:"dest_prefix"`
	ObjectCount  int64  `json:"object_count"`
	SizeBytes    int64  `json:"size_bytes"`
}

// main 解析任务名并把执行失败显式转为非零退出码。
func main() {
	if err := run(); err != nil {
		slog.Error("cron exited", slog.String("error", logging.SanitizeError(err.Error())))
		os.Exit(1)
	}
}

// run 初始化配置和基础设施,只运行明确支持的任务。
func run() error {
	task := flag.String("task", "", "cron task name")
	flag.Parse()
	if strings.TrimSpace(*task) != "backup" {
		return fmt.Errorf("未知或未实现的 cron task: %s", strings.TrimSpace(*task))
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logging.Setup(cfg.Server.LogLevel, cfg.Server.LogFormat)
	database, err := db.New(ctx, cfg.Postgres)
	if err != nil {
		return err
	}
	defer database.Close()
	objects, err := storage.New(ctx, cfg.MinIO)
	if err != nil {
		return err
	}
	ids, err := snowflake.NewNode(cfg.Snowflake.CronNodeID)
	if err != nil {
		return err
	}
	return runBackup(ctx, cfg, database, objects, ids)
}

// runBackup 执行数据库与对象存储备份,最终把结果写入 M9 backup_record。
func runBackup(ctx context.Context, cfg *config.Config, database *db.DB, objects *storage.Storage, ids snowflake.Generator) error {
	if cfg == nil || database == nil || objects == nil || ids == nil {
		return fmt.Errorf("备份任务依赖不完整")
	}
	batch := "backup-" + timex.Now().Format("20060102T150405Z")
	workDir := filepath.Join(os.TempDir(), batch)
	if err := os.MkdirAll(workDir, 0o700); err != nil {
		return fmt.Errorf("创建备份工作目录失败: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(workDir); err != nil {
			slog.Warn("backup workdir cleanup failed", slog.String("error", logging.SanitizeError(err.Error())), slog.String("path", workDir))
		}
	}()

	var sizeBytes int64
	var storageRef string
	status := admin.BackupStatusFailed
	dumpPath := filepath.Join(workDir, "postgres.dump")
	if err := dumpPostgres(ctx, cfg, dumpPath); err != nil {
		failRef, failErr := writeFailureMarker(ctx, objects, batch, err)
		if failErr != nil {
			return errors.Join(err, failErr)
		}
		if recordErr := recordBackup(ctx, database, ids, admin.BackupRecordCreate{Type: admin.BackupTypeFull, StorageRef: failRef, Status: status}); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	dbKey, err := storage.ObjectKey(0, "admin", "backup", batch, "postgres.dump")
	if err != nil {
		return err
	}
	dbSize, ref, err := uploadFile(ctx, objects, objects.BucketBackup(), dbKey, dumpPath)
	if err != nil {
		failRef, failErr := writeFailureMarker(ctx, objects, batch, err)
		if failErr != nil {
			return errors.Join(err, failErr)
		}
		if recordErr := recordBackup(ctx, database, ids, admin.BackupRecordCreate{Type: admin.BackupTypeFull, StorageRef: failRef, Status: status}); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	sizeBytes += dbSize
	objectSize, bucketSummaries, err := mirrorObjectBuckets(ctx, objects, batch)
	if err != nil {
		failRef, failErr := writeFailureMarker(ctx, objects, batch, err)
		if failErr != nil {
			return errors.Join(err, failErr)
		}
		if recordErr := recordBackup(ctx, database, ids, admin.BackupRecordCreate{Type: admin.BackupTypeFull, StorageRef: failRef, SizeBytes: sizeBytes, Status: status}); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	sizeBytes += objectSize
	storageRef, manifestSize, err := writeBackupManifest(ctx, objects, backupManifest{
		Version:   1,
		Batch:     batch,
		CreatedAt: timex.Now().UTC().Format(time.RFC3339),
		Database: backupManifestObject{
			Bucket:      objects.BucketBackup(),
			Key:         dbKey,
			ObjectRef:   ref,
			SizeBytes:   dbSize,
			ContentType: backupContentType,
		},
		ObjectBuckets:  bucketSummaries,
		TotalSizeBytes: sizeBytes,
	})
	if err != nil {
		failRef, failErr := writeFailureMarker(ctx, objects, batch, err)
		if failErr != nil {
			return errors.Join(err, failErr)
		}
		if recordErr := recordBackup(ctx, database, ids, admin.BackupRecordCreate{Type: admin.BackupTypeFull, StorageRef: failRef, SizeBytes: sizeBytes, Status: status}); recordErr != nil {
			return errors.Join(err, recordErr)
		}
		return err
	}
	sizeBytes += manifestSize
	status = admin.BackupStatusSucceeded
	if err := recordBackup(ctx, database, ids, admin.BackupRecordCreate{Type: admin.BackupTypeFull, StorageRef: storageRef, SizeBytes: sizeBytes, Status: status}); err != nil {
		return err
	}
	slog.Info("backup completed", slog.String("batch", batch), slog.Int64("size_bytes", sizeBytes))
	return nil
}

// writeBackupManifest 写入完整备份清单,backup_record.storage_ref 指向该清单而不是单个产物。
func writeBackupManifest(ctx context.Context, objects *storage.Storage, manifest backupManifest) (string, int64, error) {
	if strings.TrimSpace(manifest.Batch) == "" || strings.TrimSpace(manifest.Database.ObjectRef) == "" {
		return "", 0, fmt.Errorf("备份清单缺少批次或数据库备份引用")
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", 0, fmt.Errorf("生成备份清单失败: %w", err)
	}
	key, err := storage.ObjectKey(0, "admin", "backup", manifest.Batch, "manifest.json")
	if err != nil {
		return "", 0, err
	}
	if err := objects.Put(ctx, objects.BucketBackup(), key, bytes.NewReader(data), int64(len(data)), "application/json; charset=utf-8"); err != nil {
		return "", 0, err
	}
	ref, err := storage.ObjectRefString(objects.BucketBackup(), key)
	if err != nil {
		return "", 0, err
	}
	return ref, int64(len(data)), nil
}

// writeFailureMarker 写入失败标记对象,保证 backup_record.storage_ref 始终指向真实受控对象。
func writeFailureMarker(ctx context.Context, objects *storage.Storage, batch string, cause error) (string, error) {
	key, err := storage.ObjectKey(0, "admin", "backup", batch, "failed.txt")
	if err != nil {
		return "", err
	}
	body := []byte(logging.SanitizeError(cause.Error()))
	if err := objects.Put(ctx, objects.BucketBackup(), key, bytes.NewReader(body), int64(len(body)), "text/plain; charset=utf-8"); err != nil {
		return "", err
	}
	return storage.ObjectRefString(objects.BucketBackup(), key)
}

// dumpPostgres 使用官方 pg_dump 生成可由 pg_restore 恢复的自定义格式数据库备份。
func dumpPostgres(ctx context.Context, cfg *config.Config, outPath string) error {
	args := []string{
		"--format=custom",
		"--no-owner",
		"--no-privileges",
		"--host", cfg.Postgres.Host,
		"--port", fmt.Sprint(cfg.Postgres.Port),
		"--username", cfg.Postgres.User,
		"--dbname", cfg.Postgres.Database,
		"--file", outPath,
	}
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), "PGPASSWORD="+cfg.Postgres.Password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pg_dump 执行失败: %w: %s", err, logging.SanitizeError(string(out)))
	}
	return nil
}

// uploadFile 把本地 pg_dump 产物上传到备份桶。
func uploadFile(ctx context.Context, objects *storage.Storage, bucket, key, path string) (int64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("打开备份文件失败: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			slog.Warn("backup file close failed", slog.String("error", logging.SanitizeError(err.Error())), slog.String("path", path))
		}
	}()
	stat, err := f.Stat()
	if err != nil {
		return 0, "", fmt.Errorf("读取备份文件大小失败: %w", err)
	}
	if err := objects.Put(ctx, bucket, key, f, stat.Size(), backupContentType); err != nil {
		return 0, "", err
	}
	ref, err := storage.ObjectRefString(bucket, key)
	if err != nil {
		return 0, "", err
	}
	return stat.Size(), ref, nil
}

// mirrorObjectBuckets 把平台业务对象通过服务端复制归档到备份桶的批次前缀。
func mirrorObjectBuckets(ctx context.Context, objects *storage.Storage, batch string) (int64, []backupManifestBucket, error) {
	buckets := []string{objects.BucketCode(), objects.BucketAttach(), objects.BucketReport()}
	var total int64
	summaries := make([]backupManifestBucket, 0, len(buckets))
	for _, bucket := range buckets {
		items, errs, err := objects.ListObjects(ctx, bucket, "")
		if err != nil {
			return 0, nil, err
		}
		summary := backupManifestBucket{SourceBucket: bucket}
		for item := range items {
			dstKey, err := backupObjectKey(batch, item.Bucket, item.Key)
			if err != nil {
				return 0, nil, err
			}
			size, err := objects.CopyObject(ctx, item.Bucket, item.Key, objects.BucketBackup(), dstKey)
			if err != nil {
				return 0, nil, err
			}
			total += size
			summary.ObjectCount++
			summary.SizeBytes += size
			if summary.DestPrefix == "" {
				summary.DestPrefix, err = backupObjectPrefix(batch, item.Bucket)
				if err != nil {
					return 0, nil, err
				}
			}
		}
		for err := range errs {
			if err != nil {
				return 0, nil, err
			}
		}
		if summary.DestPrefix == "" {
			prefix, err := backupObjectPrefix(batch, bucket)
			if err != nil {
				return 0, nil, err
			}
			summary.DestPrefix = prefix
		}
		summaries = append(summaries, summary)
	}
	return total, summaries, nil
}

// backupObjectPrefix 返回某源桶在备份桶中的目标前缀。
func backupObjectPrefix(batch, bucket string) (string, error) {
	return storage.ObjectKey(0, "admin", "backup", batch, "objects", bucket)
}

// backupObjectKey 为源对象生成不会混淆路径边界的备份对象 key。
func backupObjectKey(batch, bucket, key string) (string, error) {
	parts := []string{batch, "objects", bucket}
	for _, seg := range strings.Split(key, "/") {
		if seg == "" {
			return "", storage.ErrObjectRefInvalid
		}
		parts = append(parts, url.PathEscape(seg))
	}
	return storage.ObjectKey(0, "admin", "backup", parts...)
}

// recordBackup 在 M9 自有 backup_record 表中写入本次受控备份结果。
func recordBackup(ctx context.Context, database *db.DB, ids snowflake.Generator, req admin.BackupRecordCreate) error {
	if req.StorageRef == "" {
		return fmt.Errorf("备份记录缺少对象引用")
	}
	_, err := admin.RecordBackupResult(ctx, admin.NewStore(database), ids.Generate(), req)
	return err
}
