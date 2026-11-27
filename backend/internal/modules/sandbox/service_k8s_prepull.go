package sandbox

import (
	platformk8s "chaimir/internal/platform/k8s"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
	"time"

	"chaimir/internal/platform/ids"
	"chaimir/internal/platform/workload"
	pkgcrypto "chaimir/pkg/crypto"
	"chaimir/pkg/logging"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const prepullCompositionDigestAnnotation = "chaimir.io/composition-digest"

// PrepullImage 等待本轮已受理 DaemonSet 的完整镜像闭包就绪；对象或执行版本改变即失败。
func (o *K8sOrchestrator) PrepullImage(ctx context.Context, image RuntimeImage, compositionDigest string, specs []PrepullImageSpec, progress func(PrepullResult) error) (PrepullResult, error) {
	if image.ID <= 0 || !validPrepullDigest(compositionDigest) {
		return PrepullResult{}, fmt.Errorf("预拉取镜像编号或组合摘要无效")
	}
	imageURLs := prepullImageURLs(specs)
	ds := o.prepullDaemonSet(image, compositionDigest, specs)
	cs := o.client.Clientset()
	client := cs.AppsV1().DaemonSets(o.cfg.PrepullNamespace)
	existing, err := client.Get(ctx, ds.Name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("查询预拉取 DaemonSet 失败: %w", err)
	}
	create := apierrors.IsNotFound(err)
	if !create {
		if err := verifyPrepullDaemonSet(existing, ds); err != nil {
			return PrepullResult{DaemonSet: ds.Name}, err
		}
		ds.UID = existing.UID
		ds.ResourceVersion = existing.ResourceVersion
	}
	if err := o.ensureImagePullSecrets(ctx, o.cfg.PrepullNamespace); err != nil {
		return PrepullResult{DaemonSet: ds.Name}, err
	}
	var accepted *appsv1.DaemonSet
	if create {
		accepted, err = client.Create(ctx, ds, metav1.CreateOptions{})
	} else {
		accepted, err = client.Update(ctx, ds, metav1.UpdateOptions{})
	}
	if err != nil {
		return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("保存预拉取 DaemonSet 失败: %w", err)
	}
	if err := verifyPrepullDaemonSet(accepted, ds); err != nil {
		return PrepullResult{DaemonSet: ds.Name}, err
	}
	if !samePrepullDaemonSetSpec(accepted.Spec, ds.Spec) {
		return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("预拉取 DaemonSet %s 的受理执行规格与请求不一致", ds.Name)
	}
	if !create && accepted.UID != existing.UID {
		return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("预拉取 DaemonSet %s 在更新时被替换", ds.Name)
	}
	ticker := time.NewTicker(time.Duration(o.cfg.PrepullPollIntervalSeconds) * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(time.Duration(o.cfg.PrepullTimeoutSeconds) * time.Second)
	defer timeout.Stop()
	var previous PrepullResult
	for {
		current, err := client.Get(ctx, ds.Name, metav1.GetOptions{})
		if err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("查询预拉取状态失败: %w", err)
		}
		if err := verifyPrepullDaemonSet(current, ds); err != nil {
			return PrepullResult{DaemonSet: ds.Name}, err
		}
		if current.UID != accepted.UID || current.Generation != accepted.Generation || !apiequality.Semantic.DeepEqual(current.Spec, accepted.Spec) {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("预拉取 DaemonSet %s 的对象或执行版本已改变", ds.Name)
		}
		detail, err := jsonBytes(map[string]any{
			"desired_nodes":       current.Status.DesiredNumberScheduled,
			"ready_nodes":         current.Status.NumberReady,
			"daemonset":           ds.Name,
			"generation":          current.Generation,
			"observed_generation": current.Status.ObservedGeneration,
			"uid":                 current.UID,
			"composition_digest":  compositionDigest,
			"image_count":         len(imageURLs),
			"images":              imageURLs,
		})
		if err != nil {
			return PrepullResult{DaemonSet: ds.Name}, fmt.Errorf("编码预拉取状态失败: %w", err)
		}
		result := PrepullResult{
			DesiredNodes: current.Status.DesiredNumberScheduled,
			ReadyNodes:   current.Status.NumberReady,
			DaemonSet:    ds.Name,
			Detail:       detail,
		}
		pods, err := cs.CoreV1().Pods(o.cfg.PrepullNamespace).List(ctx, metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(ds.Spec.Selector)})
		if err != nil {
			return result, fmt.Errorf("查询预拉取 Pod 状态失败: %w", err)
		}
		ownedPods := make([]corev1.Pod, 0, len(pods.Items))
		for _, pod := range pods.Items {
			owner := metav1.GetControllerOf(&pod)
			if owner != nil && owner.APIVersion == "apps/v1" && owner.Kind == "DaemonSet" && owner.Name == accepted.Name && owner.UID == accepted.UID {
				ownedPods = append(ownedPods, pod)
			}
		}
		failure := imagePullFailureFromPods(ownedPods)
		if failure == nil {
			failure = containerRuntimeFailureFromPods(ownedPods)
		}
		if failure != nil {
			detail, encodeErr := jsonBytes(map[string]any{"error": logging.SanitizeError(failure.Error()), "daemonset": ds.Name})
			if encodeErr != nil {
				return result, fmt.Errorf("编码预拉取失败详情失败: %w", encodeErr)
			}
			result.Detail = detail
			return result, failure
		}
		if progress != nil && (previous.DaemonSet == "" || previous.DesiredNodes != result.DesiredNodes || previous.ReadyNodes != result.ReadyNodes) {
			if err := progress(result); err != nil {
				return result, fmt.Errorf("保存预拉取进度失败: %w", err)
			}
			previous = result
		}
		if prepullDaemonSetReady(current) {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-timeout.C:
			return result, fmt.Errorf("预拉取等待超时")
		case <-ticker.C:
		}
	}
}

func prepullDaemonSetReady(ds *appsv1.DaemonSet) bool {
	if ds == nil || ds.Status.ObservedGeneration < ds.Generation || ds.Status.DesiredNumberScheduled <= 0 {
		return false
	}
	desired := ds.Status.DesiredNumberScheduled
	return ds.Status.UpdatedNumberScheduled == desired &&
		ds.Status.NumberReady == desired &&
		ds.Status.NumberAvailable == desired &&
		ds.Status.NumberUnavailable == 0 &&
		ds.Status.NumberMisscheduled == 0
}

// DeletePrepullDaemonSet 仅回收平台管理且身份未变的该镜像预拉取对象；NotFound 为幂等成功。
func (o *K8sOrchestrator) DeletePrepullDaemonSet(ctx context.Context, image RuntimeImage) error {
	client := o.client.Clientset().AppsV1().DaemonSets(o.cfg.PrepullNamespace)
	list, err := client.List(ctx, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(prepullManagedLabels(image)).String()})
	if err != nil {
		return fmt.Errorf("查询镜像预拉取 DaemonSet 失败: %w", err)
	}
	options := make([]metav1.DeleteOptions, len(list.Items))
	for index := range list.Items {
		item := &list.Items[index]
		digest := item.Annotations[prepullCompositionDigestAnnotation]
		if !validPrepullDigest(digest) {
			return fmt.Errorf("预拉取 DaemonSet %s 缺少有效完整摘要，拒绝回收", item.Name)
		}
		desired := prepullObjectMeta(image, digest, o.cfg.PrepullNamespace)
		if err := platformk8s.VerifyManagedLabels(item, &desired); err != nil {
			return err
		}
		if item.Name != desired.Name || item.Namespace != desired.Namespace {
			return fmt.Errorf("预拉取 DaemonSet %s 的名称或命名空间不符，拒绝回收", item.Name)
		}
		options[index], err = resourceDeleteOptions(item)
		if err != nil {
			return err
		}
	}
	for index, item := range list.Items {
		if err := client.Delete(ctx, item.Name, options[index]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("删除预拉取 DaemonSet 失败: %w", err)
		}
	}
	return nil
}

func validPrepullDigest(digest string) bool {
	value, ok := strings.CutPrefix(digest, "sha256:")
	return ok && len(value) == sha256.Size*2 && value == strings.ToLower(value) && pkgcrypto.ValidSHA256Hex(value)
}

func prepullManagedLabels(image RuntimeImage) map[string]string {
	return map[string]string{"app": "chaimir", "module": "sandbox", "chaimir.io/managed-by": "chaimir-backend", "runtime_image_id": ids.Format(image.ID)}
}

func prepullObjectMeta(image RuntimeImage, digest, namespace string) metav1.ObjectMeta {
	prefix := strings.TrimPrefix(digest, "sha256:")
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	labels := prepullManagedLabels(image)
	labels["composition_digest"] = prefix
	return metav1.ObjectMeta{
		Name:        "chaimir-prepull-" + ids.Format(image.ID) + "-" + prefix,
		Namespace:   namespace,
		Labels:      labels,
		Annotations: map[string]string{prepullCompositionDigestAnnotation: digest},
	}
}

func verifyPrepullDaemonSet(current, desired *appsv1.DaemonSet) error {
	if err := platformk8s.VerifyManagedLabels(current, desired); err != nil {
		return err
	}
	if current.Name != desired.Name || current.Namespace != desired.Namespace || current.Annotations[prepullCompositionDigestAnnotation] != desired.Annotations[prepullCompositionDigestAnnotation] {
		return fmt.Errorf("预拉取 DaemonSet %s 的身份或完整组合摘要不符", desired.Name)
	}
	if current.UID == "" || current.ResourceVersion == "" || current.Generation <= 0 || current.DeletionTimestamp != nil {
		return fmt.Errorf("预拉取 DaemonSet %s 的身份不完整或正在删除", desired.Name)
	}
	return nil
}

func samePrepullDaemonSetSpec(current, desired appsv1.DaemonSetSpec) bool {
	return apiequality.Semantic.DeepEqual(normalizePrepullDaemonSetSpec(current), normalizePrepullDaemonSetSpec(desired))
}

func normalizePrepullDaemonSetSpec(spec appsv1.DaemonSetSpec) appsv1.DaemonSetSpec {
	spec = *spec.DeepCopy()
	pod := &spec.Template.Spec
	normalized := platformk8s.NormalizePodExecutionSpec(*pod)
	// 模板尚未调度；固定节点和开销属于请求规格，不能按已调度 Pod 忽略。
	normalized.NodeName, normalized.Overhead = pod.NodeName, pod.Overhead
	*pod = normalized
	if spec.RevisionHistoryLimit == nil {
		limit := int32(10)
		spec.RevisionHistoryLimit = &limit
	}
	if spec.UpdateStrategy.Type == "" {
		spec.UpdateStrategy.Type = appsv1.RollingUpdateDaemonSetStrategyType
	}
	if spec.UpdateStrategy.Type == appsv1.RollingUpdateDaemonSetStrategyType {
		if spec.UpdateStrategy.RollingUpdate == nil {
			spec.UpdateStrategy.RollingUpdate = &appsv1.RollingUpdateDaemonSet{}
		}
		rolling := spec.UpdateStrategy.RollingUpdate
		if rolling.MaxUnavailable == nil {
			value := intstr.FromInt32(1)
			rolling.MaxUnavailable = &value
		}
		if rolling.MaxSurge == nil {
			value := intstr.FromInt32(0)
			rolling.MaxSurge = &value
		}
	}
	return spec
}

func (o *K8sOrchestrator) prepullDaemonSet(image RuntimeImage, compositionDigest string, specs []PrepullImageSpec) *appsv1.DaemonSet {
	metadata := prepullObjectMeta(image, compositionDigest, o.cfg.PrepullNamespace)
	automount := false
	initContainers := make([]corev1.Container, 0, len(specs))
	for idx, spec := range specs {
		initContainers = append(initContainers, corev1.Container{
			Name:            prepullContainerName(idx, spec.ImageURL),
			Image:           spec.ImageURL,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Command:         spec.Command,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullRequestCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullRequestMemory)},
				Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullLimitCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullLimitMemory)},
			},
			SecurityContext: containerSecurityContext(true),
			VolumeMounts:    prepullVolumeMounts(idx, spec),
		})
	}
	holdSpec := prepullHoldSpec(specs)
	hold := corev1.Container{
		Name:            "prepull-ready",
		Image:           holdSpec.ImageURL,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"sleep", "2147483647"},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullRequestCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullRequestMemory)},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(o.cfg.PrepullLimitCPU), corev1.ResourceMemory: resource.MustParse(o.cfg.PrepullLimitMemory)},
		},
		SecurityContext: containerSecurityContext(true),
	}
	return &appsv1.DaemonSet{
		ObjectMeta: metadata,
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: metadata.Labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: metadata.Labels, Annotations: metadata.Annotations},
				Spec: corev1.PodSpec{
					AutomountServiceAccountToken: &automount,
					EnableServiceLinks:           &automount,
					RestartPolicy:                corev1.RestartPolicyAlways,
					ImagePullSecrets:             imagePullSecrets(o.cfg.ImagePullSecretNames),
					NodeSelector:                 copyStringMap(o.cfg.SandboxNodeSelector),
					Tolerations:                  sandboxTolerations(o.cfg.SandboxNodeTolerations),
					InitContainers:               initContainers,
					Containers:                   []corev1.Container{hold},
					SecurityContext:              podSecurityContext(),
					Volumes:                      prepullVolumes(initContainers),
				},
			},
		},
	}
}

// prepullVolumeMounts 提供镜像自检所需的最小临时工作目录,并复用镜像清单声明的临时可写目录。
func prepullVolumeMounts(index int, spec PrepullImageSpec) []corev1.VolumeMount {
	prefix := fmt.Sprintf("prepull-%d-", index)
	mounts := []corev1.VolumeMount{
		{Name: prefix + "workspace", MountPath: "/workspace"},
		{Name: prefix + "tmp", MountPath: "/tmp"},
	}
	seenPaths := map[string]struct{}{"/workspace": {}, "/tmp": {}}
	for _, mount := range spec.EphemeralMounts {
		mountPath := path.Clean(strings.TrimSpace(mount.MountPath))
		if !strings.HasPrefix(mountPath, "/") {
			continue
		}
		if _, exists := seenPaths[mountPath]; exists {
			continue
		}
		seenPaths[mountPath] = struct{}{}
		mounts = append(mounts, corev1.VolumeMount{
			Name:      boundedSandboxName(prefix+prepullExtraVolumeName(mount), 63),
			MountPath: mountPath,
		})
	}
	return mounts
}

func prepullVolumes(containers []corev1.Container) []corev1.Volume {
	var volumes []corev1.Volume
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			volumes = append(volumes, corev1.Volume{
				Name:         mount.Name,
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			})
		}
	}
	return volumes
}

func prepullExtraVolumeName(mount workload.EphemeralMountSpec) string {
	key := strings.TrimSpace(mount.Name) + "\x00" + strings.TrimSpace(mount.MountPath)
	sum := sha256.Sum256([]byte(key))
	suffix := hex.EncodeToString(sum[:4])
	base := "prepull-" + strings.TrimSpace(mount.Name)
	base = strings.ToLower(base)
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	base = strings.Trim(b.String(), "-")
	if base == "" || base == "prepull" {
		base = "prepull-mount"
	}
	maxBase := 63 - len(suffix) - 1
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return strings.TrimRight(base, "-") + "-" + suffix
}

func prepullHoldSpec(specs []PrepullImageSpec) PrepullImageSpec {
	for _, spec := range specs {
		if spec.Hold {
			return spec
		}
	}
	return PrepullImageSpec{}
}

func prepullContainerName(index int, imageURL string) string {
	sum := sha256.Sum256([]byte(imageURL))
	name := strings.NewReplacer("/", "-", ":", "-", "@", "-").Replace(strings.TrimSpace(imageURL))
	name = strings.ToLower(name)
	if len(name) > 36 {
		name = name[len(name)-36:]
	}
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "image"
	}
	return fmt.Sprintf("prepull-%02d-%s-%s", index, name, hex.EncodeToString(sum[:3]))
}
