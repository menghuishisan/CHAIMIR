// Package storage 封装 MinIO 客户端,提供统一对象存储服务。
// 依据 docs/总-技术选型.md §4 + 蓝图 §3:路径 {tenant_id}/{模块}/{资源类型}/{资源id}/...。
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

// ErrObjectRefInvalid 表示对象存储引用不是 minio://bucket/key 格式。
var ErrObjectRefInvalid = errors.New("对象存储引用格式非法")

// Storage 封装 MinIO 客户端与桶配置。
type Storage struct {
	client       *minio.Client
	bucketCode   string
	bucketAttach string
	bucketReport string
	bucketBackup string
}

// New 创建 MinIO 客户端并执行启动期连通性检查,对象存储不可用时服务必须 fail-fast。
func New(ctx context.Context, cfg config.MinIOConfig) (*Storage, error) {
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

// EnsureBuckets 启动时确保所需桶存在(幂等)。
func (s *Storage) EnsureBuckets(ctx context.Context) error {
	for _, b := range []string{s.bucketCode, s.bucketAttach, s.bucketReport, s.bucketBackup} {
		exists, err := s.client.BucketExists(ctx, b)
		if err != nil {
			return fmt.Errorf("检查桶 %s 失败: %w", b, err)
		}
		if !exists {
			if err := s.client.MakeBucket(ctx, b, minio.MakeBucketOptions{}); err != nil {
				return fmt.Errorf("创建桶 %s 失败: %w", b, err)
			}
		}
	}
	return nil
}

// ObjectKey 按统一约定拼对象 key:{tenant_id}/{module}/{resourceType}/{parts...},并拒绝会混淆命名空间的段。
func ObjectKey(tenantID int64, module, resourceType string, parts ...string) (string, error) {
	segs := append([]string{strconv.FormatInt(tenantID, 10), module, resourceType}, parts...)
	for _, seg := range segs {
		if strings.TrimSpace(seg) == "" || seg == "." || seg == ".." || strings.Contains(seg, "/") || strings.Contains(seg, "\\") {
			return "", fmt.Errorf("对象 key 段不安全: %q", seg)
		}
	}
	return strings.Join(segs, "/"), nil
}

// Put 上传对象到已鉴权生成的 bucket/key,平台层只负责传输不接受业务侧直链语义。
func (s *Storage) Put(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error {
	if _, err := s.client.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType}); err != nil {
		return fmt.Errorf("上传对象 %s/%s 失败: %w", bucket, key, err)
	}
	return nil
}

// Get 返回对象读取流;下载鉴权和 bucket/key 选择必须在调用模块完成。
func (s *Storage) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("下载对象 %s/%s 失败: %w", bucket, key, err)
	}
	return obj, nil
}

// BucketCode 返回学生代码和沙箱归档使用的对象桶。
func (s *Storage) BucketCode() string { return s.bucketCode }

// BucketAttach 返回通用附件使用的对象桶。
func (s *Storage) BucketAttach() string { return s.bucketAttach }

// BucketReport 返回实验报告和成绩单等报告类文件使用的对象桶。
func (s *Storage) BucketReport() string { return s.bucketReport }

// BucketBackup 返回备份归档使用的对象桶。
func (s *Storage) BucketBackup() string { return s.bucketBackup }
