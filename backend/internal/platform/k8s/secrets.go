// k8s secrets 文件提供跨运行引擎复用的镜像拉取凭据同步能力。
package k8s

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SyncImagePullSecrets 把受控源命名空间的镜像凭据最小复制到动态工作负载命名空间。
func (c *Client) SyncImagePullSecrets(ctx context.Context, sourceNamespace, targetNamespace, module string, names []string) error {
	if c == nil || c.clientset == nil {
		return fmt.Errorf("K8s 客户端未初始化")
	}
	sourceNamespace = strings.TrimSpace(sourceNamespace)
	targetNamespace = strings.TrimSpace(targetNamespace)
	module = strings.TrimSpace(module)
	if sourceNamespace == "" || targetNamespace == "" || module == "" {
		return fmt.Errorf("镜像拉取 Secret 同步边界不完整")
	}
	if sourceNamespace == targetNamespace || len(names) == 0 {
		return nil
	}
	sourceClient := c.clientset.CoreV1().Secrets(sourceNamespace)
	for _, rawName := range names {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return fmt.Errorf("镜像拉取 Secret 名称不能为空")
		}
		source, err := sourceClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("查询镜像拉取 Secret %s/%s 失败: %w", sourceNamespace, name, err)
		}
		if source.Type != corev1.SecretTypeDockerConfigJson && source.Type != corev1.SecretTypeDockercfg {
			return fmt.Errorf("镜像拉取 Secret %s/%s 类型无效", sourceNamespace, name)
		}
		desired := imagePullSecretForNamespace(source, targetNamespace, module)
		if err := c.syncManagedSecret(ctx, desired); err != nil {
			return fmt.Errorf("同步镜像拉取 Secret %s/%s 失败: %w", targetNamespace, name, err)
		}
	}
	return nil
}

// imagePullSecretForNamespace 构造不携带源对象元数据的最小目标 Secret。
func imagePullSecretForNamespace(source *corev1.Secret, namespace, module string) *corev1.Secret {
	data := make(map[string][]byte, len(source.Data))
	for key, value := range source.Data {
		data[key] = bytes.Clone(value)
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: source.Name, Namespace: namespace, Labels: map[string]string{
			"app": "chaimir", "module": module, "chaimir.io/managed-by": "chaimir-backend", "chaimir.io/purpose": "image-pull",
		}},
		Type: source.Type,
		Data: data,
	}
}

// sameSecretContent 核对类型与全部数据键值，不在错误中包含密钥内容。
func sameSecretContent(current, desired *corev1.Secret) bool {
	if current.Type != desired.Type || len(current.Data) != len(desired.Data) {
		return false
	}
	for key, value := range desired.Data {
		if !bytes.Equal(current.Data[key], value) {
			return false
		}
	}
	return true
}

// SyncSecretKeys 将组合实际声明的敏感键从平台 Secret 同步到沙箱命名空间。
// 只复制请求键,避免把控制面其它凭据暴露给学生工作负载。
func (c *Client) SyncSecretKeys(ctx context.Context, sourceNamespace, sourceName, targetNamespace, targetName, module string, keys []string) error {
	if c == nil || c.clientset == nil {
		return fmt.Errorf("K8s 客户端未初始化")
	}
	sourceNamespace = strings.TrimSpace(sourceNamespace)
	sourceName = strings.TrimSpace(sourceName)
	targetNamespace = strings.TrimSpace(targetNamespace)
	targetName = strings.TrimSpace(targetName)
	module = strings.TrimSpace(module)
	if sourceNamespace == "" || sourceName == "" || targetNamespace == "" || targetName == "" || module == "" {
		return fmt.Errorf("运行期 Secret 同步边界不完整")
	}
	if sourceNamespace == targetNamespace && sourceName == targetName {
		return fmt.Errorf("运行期 Secret 不能覆盖控制面源对象")
	}
	unique := make(map[string]struct{}, len(keys))
	for _, raw := range keys {
		key := strings.TrimSpace(raw)
		if key == "" {
			return fmt.Errorf("运行期 Secret 键不能为空")
		}
		unique[key] = struct{}{}
	}
	if len(unique) == 0 {
		return nil
	}
	source, err := c.clientset.CoreV1().Secrets(sourceNamespace).Get(ctx, sourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("查询运行期 Secret %s/%s 失败: %w", sourceNamespace, sourceName, err)
	}
	data := make(map[string][]byte, len(unique))
	for key := range unique {
		value, ok := source.Data[key]
		if !ok || len(value) == 0 {
			return fmt.Errorf("运行期 Secret %s 缺少键 %s", sourceName, key)
		}
		data[key] = bytes.Clone(value)
	}
	desired := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: targetName, Namespace: targetNamespace, Labels: map[string]string{
			"app": "chaimir", "module": module, "chaimir.io/managed-by": "chaimir-backend", "chaimir.io/purpose": "sandbox-runtime",
		}},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := c.syncManagedSecret(ctx, desired); err != nil {
		return fmt.Errorf("同步沙箱运行期 Secret %s/%s 失败: %w", targetNamespace, targetName, err)
	}
	return nil
}

func (c *Client) syncManagedSecret(ctx context.Context, desired *corev1.Secret) error {
	client := c.clientset.CoreV1().Secrets(desired.Namespace)
	existing, err := client.Get(ctx, desired.Name, metav1.GetOptions{})
	created := false
	if apierrors.IsNotFound(err) {
		existing, err = client.Create(ctx, desired, metav1.CreateOptions{})
		created = err == nil
		if apierrors.IsAlreadyExists(err) {
			existing, err = client.Get(ctx, desired.Name, metav1.GetOptions{})
		}
	}
	if err != nil {
		return err
	}
	if err := VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	if !created && !sameSecretContent(existing, desired) {
		updated := existing.DeepCopy()
		updated.Type, updated.Data = desired.Type, desired.Data
		accepted, err := client.Update(ctx, updated, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		if accepted.UID != existing.UID {
			return fmt.Errorf("受管 Secret %s 在更新期间已被替换", desired.Name)
		}
		existing = accepted
	}
	if err := VerifyManagedResource(existing, desired); err != nil {
		return err
	}
	if !sameSecretContent(existing, desired) {
		return fmt.Errorf("受管 Secret %s 的类型或数据与请求不一致", desired.Name)
	}
	return nil
}
