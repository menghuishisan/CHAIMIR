// M2 镜像安全门禁:统一校验运行时与工具镜像的仓库、digest、签名和扫描状态。
package sandbox

import (
	"fmt"
	"regexp"
	"strings"

	"chaimir/internal/platform/config"
	"chaimir/pkg/apperr"
)

const (
	// ImageScanPassed 表示 Trivy/Harbor 安全扫描已通过。
	ImageScanPassed = "passed"
)

var imageDigestPattern = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)

// ImageSecuritySpec 是 CI/Harbor 门禁写入控制面的不可变镜像安全证明。
type ImageSecuritySpec struct {
	ImageURL string
	Digest   string
}

// validateImageSecurityGate 要求镜像来自私有仓库、使用 digest 引用,且匹配受控 CI/Harbor 证明清单。
func validateImageSecurityGate(spec ImageSecuritySpec, cfg config.SandboxConfig) error {
	if err := validateRuntimeImageURL(spec.ImageURL, cfg); err != nil {
		return err
	}
	digest := strings.TrimSpace(spec.Digest)
	if !imageDigestPattern.MatchString(digest) || !strings.Contains(spec.ImageURL, "@"+digest) {
		return apperr.ErrRuntimePrepullFailed.WithCause(fmt.Errorf("镜像必须使用匹配的 sha256 digest 引用"))
	}
	attestation, ok := configuredImageAttestation(spec.ImageURL, digest, cfg)
	if !ok {
		return apperr.ErrRuntimePrepullFailed.WithCause(fmt.Errorf("镜像缺少受控安全证明"))
	}
	if !attestation.CosignVerified {
		return apperr.ErrRuntimePrepullFailed.WithCause(fmt.Errorf("镜像签名未通过验证"))
	}
	if strings.ToLower(strings.TrimSpace(attestation.TrivyStatus)) != ImageScanPassed {
		return apperr.ErrRuntimePrepullFailed.WithCause(fmt.Errorf("镜像安全扫描未通过"))
	}
	return nil
}

// configuredImageAttestation 按 image_url 与 digest 精确匹配受控证明,避免信任 HTTP 请求字段。
func configuredImageAttestation(imageURL, digest string, cfg config.SandboxConfig) (config.SandboxImageAttestation, bool) {
	for _, item := range cfg.ImageAttestations {
		if strings.TrimSpace(item.ImageURL) == strings.TrimSpace(imageURL) &&
			strings.TrimSpace(item.Digest) == digest {
			return item, true
		}
	}
	return config.SandboxImageAttestation{}, false
}
