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
	targetClient := c.clientset.CoreV1().Secrets(targetNamespace)
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
		existing, err := targetClient.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			if _, err := targetClient.Create(ctx, desired, metav1.CreateOptions{}); err != nil {
				return fmt.Errorf("创建镜像拉取 Secret %s/%s 失败: %w", targetNamespace, name, err)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("查询镜像拉取 Secret %s/%s 失败: %w", targetNamespace, name, err)
		}
		if sameImagePullSecret(existing, desired) {
			continue
		}
		updated := existing.DeepCopy()
		updated.Type = desired.Type
		updated.Data = desired.Data
		updated.Labels = desired.Labels
		if _, err := targetClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("更新镜像拉取 Secret %s/%s 失败: %w", targetNamespace, name, err)
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

// sameImagePullSecret 判断目标凭据是否已经与受控源一致。
func sameImagePullSecret(current, desired *corev1.Secret) bool {
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
	targetClient := c.clientset.CoreV1().Secrets(targetNamespace)
	existing, err := targetClient.Get(ctx, targetName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		if _, err := targetClient.Create(ctx, desired, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("创建沙箱运行期 Secret %s/%s 失败: %w", targetNamespace, targetName, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("查询沙箱运行期 Secret %s/%s 失败: %w", targetNamespace, targetName, err)
	}
	updated := existing.DeepCopy()
	updated.Type = desired.Type
	updated.Data = desired.Data
	updated.Labels = desired.Labels
	if _, err := targetClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("更新沙箱运行期 Secret %s/%s 失败: %w", targetNamespace, targetName, err)
	}
	return nil
}
