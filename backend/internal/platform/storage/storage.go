// storage 封装 MinIO 客户端与统一对象键规则,供代码、附件、报告和备份复用。
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"chaimir/internal/platform/config"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrObjectRefInvalid 表示对象引用不是受支持的 minio://bucket/key 格式。
var ErrObjectRefInvalid = errors.New("对象存储引用格式非法")

// Storage 封装 MinIO 客户端与平台约定桶名。
type Storage struct {
	client       *minio.Client
	bucketCode   string
	bucketAttach string
	bucketReport string
	bucketBackup string
}

// ObjectInfo 是统一对象存储对外暴露的安全对象摘要。
type ObjectInfo struct {
	Bucket string
	Key    string
	Size   int64
}

// TenantQuota 表示统一文件服务执行上传前校验所需的租户文件配额快照。
type TenantQuota struct {
	MaxFiles  int64
	MaxBytes  int64
	UsedFiles int64
	UsedBytes int64
}

// New 创建 MinIO 客户端并执行启动期连通性检查。
func New(ctx context.Context, cfg config.MinIOConfig) (*Storage, error) {
	for _, bucket := range []string{cfg.BucketCode, cfg.BucketAttach, cfg.BucketReport, cfg.BucketBackup} {
		if !safeObjectRefBucket(bucket) {
			return nil, fmt.Errorf("对象存储桶名非法: %s", bucket)
		}
	}
	if cfg.PingTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("MINIO_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("创建 MinIO 客户端失败: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.PingTimeoutSeconds)*time.Second)
	defer cancel()
	if _, err := client.ListBuckets(pingCtx); err != nil {
		return nil, fmt.Errorf("MinIO 连通性检查失败: %w", err)
	}
	return &Storage{
		client:       client,
		bucketCode:   cfg.BucketCode,
		bucketAttach: cfg.BucketAttach,
		bucketReport: cfg.BucketReport,
		bucketBackup: cfg.BucketBackup,
	}, nil
}

// EnsureBuckets 幂等确保平台所需桶存在。
func (s *Storage) EnsureBuckets(ctx context.Context) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("对象存储客户端未初始化")
	}
	for _, bucket := range []string{s.bucketCode, s.bucketAttach, s.bucketReport, s.bucketBackup} {
		if !safeObjectRefBucket(bucket) {
			return ErrObjectRefInvalid
		}
		exists, err := s.client.BucketExists(ctx, bucket)
		if err != nil {
			return fmt.Errorf("检查桶 %s 失败: %w", bucket, err)
		}
		if !exists {
			if err := s.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("创建桶 %s 失败: %w", bucket, err)
			}
		}
	}
	return nil
}

// AllowUpload 根据租户文件数和总字节数配额判断一次上传是否允许进入后续链路。
func (q TenantQuota) AllowUpload(fileCount, totalBytes int64) error {
	if fileCount <= 0 {
		return fmt.Errorf("上传文件数必须大于 0")
	}
	if totalBytes <= 0 {
		return fmt.Errorf("上传字节数必须大于 0")
	}
	if q.MaxFiles > 0 && q.UsedFiles+fileCount > q.MaxFiles {
		return fmt.Errorf("租户文件数量超出配额")
	}
	if q.MaxBytes > 0 && q.UsedBytes+totalBytes > q.MaxBytes {
		return fmt.Errorf("租户文件总字节数超出配额")
	}
	return nil
}

// ObjectKey 按统一约定生成对象 key:{tenant_id}/{module}/{resourceType}/{parts...}。
func ObjectKey(tenantID int64, module, resourceType string, parts ...string) (string, error) {
	if tenantID < 0 {
		return "", fmt.Errorf("对象 key 缺少租户或平台作用域")
	}
	segs := append([]string{strconv.FormatInt(tenantID, 10), module, resourceType}, parts...)
	for _, seg := range segs {
		if seg != strings.TrimSpace(seg) || seg == "" || seg == "." || seg == ".." || strings.Contains(seg, "/") || strings.Contains(seg, "\\") {
			return "", fmt.Errorf("对象 key 段不安全: %q", seg)
		}
	}
	return strings.Join(segs, "/"), nil
}

// Put 上传对象到指定 bucket/key。
func (s *Storage) Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("对象存储客户端未初始化")
	}
	if r == nil || size <= 0 {
		return fmt.Errorf("上传对象内容不能为空")
	}
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return ErrObjectRefInvalid
	}
	if _, err := s.client.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("上传对象 %s/%s 失败: %w", bucket, key, err)
	}
	return nil
}

// Get 打开对象读取流。
func (s *Storage) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("对象存储客户端未初始化")
	}
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return nil, ErrObjectRefInvalid
	}
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("下载对象 %s/%s 失败: %w", bucket, key, err)
	}
	return obj, nil
}

// OpenDownload 打开下载对象并读取可信长度和内容类型,供统一下载入口流式响应。
func (s *Storage) OpenDownload(ctx context.Context, bucket, key string) (io.ReadCloser, int64, string, error) {
	if s == nil || s.client == nil {
		return nil, 0, "", fmt.Errorf("对象存储客户端未初始化")
	}
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return nil, 0, "", ErrObjectRefInvalid
	}
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, "", fmt.Errorf("打开下载对象 %s/%s 失败: %w", bucket, key, err)
	}
	info, err := obj.Stat()
	if err != nil {
		if closeErr := obj.Close(); closeErr != nil {
			return nil, 0, "", fmt.Errorf("读取下载对象 %s/%s 信息失败: %w; 关闭对象失败: %v", bucket, key, err, closeErr)
		}
		return nil, 0, "", fmt.Errorf("读取下载对象 %s/%s 信息失败: %w", bucket, key, err)
	}
	return obj, info.Size, info.ContentType, nil
}

// Delete 删除指定 bucket/key 对象,供业务在跨资源事务失败时清理已写对象。
func (s *Storage) Delete(ctx context.Context, bucket, key string) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("对象存储客户端未初始化")
	}
	if !safeObjectRefBucket(bucket) || !safeObjectRefKey(key) {
		return ErrObjectRefInvalid
	}
	if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("删除对象 %s/%s 失败: %w", bucket, key, err)
	}
	return nil
}

// ListObjects 流式枚举指定 bucket 下的安全对象摘要,供受控后台任务生成备份清单。
func (s *Storage) ListObjects(ctx context.Context, bucket, prefix string) (<-chan ObjectInfo, <-chan error, error) {
	if s == nil || s.client == nil {
		return nil, nil, fmt.Errorf("对象存储客户端未初始化")
	}
	if !safeObjectRefBucket(bucket) {
		return nil, nil, ErrObjectRefInvalid
	}
	if prefix != "" && !safeObjectRefKey(strings.TrimSuffix(prefix, "/")) {
		return nil, nil, ErrObjectRefInvalid
	}
	out := make(chan ObjectInfo)
	errs := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errs)
		for obj := range s.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if obj.Err != nil {
				errs <- fmt.Errorf("枚举对象 %s 失败: %w", bucket, obj.Err)
				return
			}
			if obj.Key == "" || !safeObjectRefKey(obj.Key) {
				errs <- ErrObjectRefInvalid
				return
			}
			out <- ObjectInfo{Bucket: bucket, Key: obj.Key, Size: obj.Size}
		}
	}()
	return out, errs, nil
}

// CopyObject 在对象存储服务端复制对象,用于备份任务避免把大对象落到本地磁盘。
func (s *Storage) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) (int64, error) {
	if s == nil || s.client == nil {
		return 0, fmt.Errorf("对象存储客户端未初始化")
	}
	if !safeObjectRefBucket(srcBucket) || !safeObjectRefBucket(dstBucket) || !safeObjectRefKey(srcKey) || !safeObjectRefKey(dstKey) {
		return 0, ErrObjectRefInvalid
	}
	info, err := s.client.CopyObject(ctx, minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}, minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey})
	if err != nil {
		return 0, fmt.Errorf("复制对象 %s/%s 到 %s/%s 失败: %w", srcBucket, srcKey, dstBucket, dstKey, err)
	}
	return info.Size, nil
}

// BucketCode 返回代码桶名。
func (s *Storage) BucketCode() string { return s.bucketCode }

// BucketAttach 返回附件桶名。
func (s *Storage) BucketAttach() string { return s.bucketAttach }

// BucketReport 返回报告桶名。
func (s *Storage) BucketReport() string { return s.bucketReport }

// BucketBackup 返回备份桶名。
func (s *Storage) BucketBackup() string { return s.bucketBackup }
