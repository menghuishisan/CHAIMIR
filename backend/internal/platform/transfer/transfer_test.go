// transfer_test 用聚焦测试守住统一导入导出中心的状态机和下载中心语义。
package transfer

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestManagerNewTaskBuildsPendingTask 确认统一导入导出中心会创建统一 pending 任务快照。
func TestManagerNewTaskBuildsPendingTask(t *testing.T) {
	manager := Manager{
		Config: Config{
			MaxAttempts:      3,
			RetryDelay:       time.Second,
			DownloadGrantTTL: 15 * time.Minute,
		},
	}
	task, err := manager.NewTask(NewTaskRequest{
		TaskID:      "task-1",
		TenantID:    42,
		AccountID:   1001,
		Channel:     ChannelExport,
		Subject:     "grade.transcript",
		FileName:    "transcript.zip",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	if task.Status != StatusPending {
		t.Fatalf("status = %s, want pending", task.Status)
	}
	if task.MaxAttempts != 3 {
		t.Fatalf("max attempts = %d", task.MaxAttempts)
	}
}

// TestManagerCompleteTaskPublishesArtifactRef 确认统一导入导出中心完成后只记录统一文件服务对象引用。
func TestManagerCompleteTaskPublishesArtifactRef(t *testing.T) {
	manager := Manager{
		Config: Config{
			MaxAttempts:      3,
			RetryDelay:       time.Second,
			DownloadGrantTTL: 15 * time.Minute,
		},
	}
	task, err := manager.NewTask(NewTaskRequest{
		TaskID:      "task-1",
		TenantID:    42,
		AccountID:   1001,
		Channel:     ChannelExport,
		Subject:     "grade.transcript",
		FileName:    "transcript.zip",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	completed, err := manager.CompleteTask(task, CompleteTaskRequest{
		ObjectRef: "minio://chaimir-report/42/transfer/export/task-1/transcript.zip",
		Size:      2048,
	})
	if err != nil {
		t.Fatalf("complete task: %v", err)
	}
	if completed.Status != StatusSucceeded {
		t.Fatalf("status = %s, want succeeded", completed.Status)
	}
	if completed.Artifact.ObjectRef != "minio://chaimir-report/42/transfer/export/task-1/transcript.zip" {
		t.Fatalf("artifact ref = %q", completed.Artifact.ObjectRef)
	}
}

// TestManagerFailTaskRetriesBeforeFinalFailure 确认统一导入导出中心会先重试,重试耗尽后才进入 failed。
func TestManagerFailTaskRetriesBeforeFinalFailure(t *testing.T) {
	manager := Manager{
		Config: Config{
			MaxAttempts:      2,
			RetryDelay:       time.Second,
			DownloadGrantTTL: 15 * time.Minute,
		},
	}
	task, err := manager.NewTask(NewTaskRequest{
		TaskID:      "task-1",
		TenantID:    42,
		AccountID:   1001,
		Channel:     ChannelExport,
		Subject:     "grade.transcript",
		FileName:    "transcript.zip",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	retrying, err := manager.FailTask(task, errors.New("temporary error"), time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("fail task first time: %v", err)
	}
	if retrying.Status != StatusRetrying {
		t.Fatalf("status = %s, want retrying", retrying.Status)
	}
	if retrying.NextAttemptAfter.IsZero() {
		t.Fatalf("retry task should populate next attempt time")
	}

	failed, err := manager.FailTask(retrying, errors.New("temporary error"), time.Date(2026, 6, 9, 10, 0, 1, 0, time.UTC))
	if err != nil {
		t.Fatalf("fail task second time: %v", err)
	}
	if failed.Status != StatusFailed {
		t.Fatalf("status = %s, want failed", failed.Status)
	}
}

// TestManagerBuildDownloadGrantRejectsNonSucceededTask 确认下载中心只为已完成且有产物的任务签发下载授权。
func TestManagerBuildDownloadGrantRejectsNonSucceededTask(t *testing.T) {
	manager := Manager{
		Config: Config{
			MaxAttempts:      3,
			RetryDelay:       time.Second,
			DownloadGrantTTL: 15 * time.Minute,
		},
		StorageSigningKey: strings.Repeat("k", 32),
	}
	task, err := manager.NewTask(NewTaskRequest{
		TaskID:      "task-1",
		TenantID:    42,
		AccountID:   1001,
		Channel:     ChannelExport,
		Subject:     "grade.transcript",
		FileName:    "transcript.zip",
		ContentType: "application/zip",
	})
	if err != nil {
		t.Fatalf("new task: %v", err)
	}
	if _, _, err := manager.BuildDownloadGrant(task, time.Date(2026, 6, 9, 10, 0, 0, 0, time.UTC)); err == nil {
		t.Fatalf("pending task should not issue download grant")
	}
}
