package k8s

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VerifyManagedLabels 核对请求声明的全部管理标签，防止复用其他执行者的同名资源。
func VerifyManagedLabels(existing, desired metav1.Object) error {
	for key, value := range desired.GetLabels() {
		if existing.GetLabels()[key] != value {
			return fmt.Errorf("对象 %s 的管理标签 %s 不一致", desired.GetName(), key)
		}
	}
	return nil
}

// VerifyManagedResource 核对 API 受理或复用对象的身份、生命周期与管理归属。
// 对象规格及更新前后 UID 的一致性仍由拥有其执行契约的调用者核验。
func VerifyManagedResource(existing, desired metav1.Object) error {
	if existing.GetName() != desired.GetName() || existing.GetNamespace() != desired.GetNamespace() || existing.GetUID() == "" || existing.GetDeletionTimestamp() != nil {
		return fmt.Errorf("对象 %s 身份无效或正在删除", desired.GetName())
	}
	return VerifyManagedLabels(existing, desired)
}
