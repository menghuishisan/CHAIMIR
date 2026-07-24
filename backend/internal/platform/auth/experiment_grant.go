// auth experiment_grant 文件实现 M6 与 M7 之间无同步依赖的短时实验启动授权。
package auth

import (
	"errors"
	"fmt"

	"chaimir/internal/platform/timex"

	"github.com/golang-jwt/jwt/v5"
)

const experimentLaunchGrantType = "experiment_launch"

// ExperimentLaunchGrantClaims 绑定一次实验启动所需的租户、账号、实验和可选课时边界。
type ExperimentLaunchGrantClaims struct {
	TenantID     int64  `json:"tid"`
	AccountID    int64  `json:"aid"`
	ExperimentID int64  `json:"eid"`
	LessonID     int64  `json:"lid,omitempty"`
	Type         string `json:"typ"`
	jwt.RegisteredClaims
}

// IssueExperimentLaunchGrant 签发账号绑定的短时实验启动授权；lessonID 为 0 表示独立实验入口。
func (m *Manager) IssueExperimentLaunchGrant(tenantID, accountID, experimentID, lessonID int64) (string, error) {
	if m == nil || tenantID <= 0 || accountID <= 0 || experimentID <= 0 || lessonID < 0 || m.experimentLaunchTTL <= 0 {
		return "", errors.New("实验启动授权载荷不完整")
	}
	now := timex.Now()
	claims := ExperimentLaunchGrantClaims{
		TenantID: tenantID, AccountID: accountID, ExperimentID: experimentID, LessonID: lessonID, Type: experimentLaunchGrantType,
		RegisteredClaims: jwt.RegisteredClaims{Issuer: m.issuer, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(m.experimentLaunchTTL))},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.hmacKey)
	if err != nil {
		return "", fmt.Errorf("签发实验启动授权失败: %w", err)
	}
	return token, nil
}

// VerifyExperimentLaunchGrant 校验授权签名、有效期及其与当前请求身份和实验的一致性。
func (m *Manager) VerifyExperimentLaunchGrant(tokenString string, tenantID, accountID, experimentID int64) error {
	if m == nil || tenantID <= 0 || accountID <= 0 || experimentID <= 0 {
		return errors.New("实验启动请求身份不完整")
	}
	claims := &ExperimentLaunchGrantClaims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("非预期签名算法: %v", token.Header["alg"])
		}
		return m.hmacKey, nil
	}, jwt.WithIssuer(m.issuer))
	if err != nil {
		return fmt.Errorf("实验启动授权校验失败: %w", err)
	}
	if claims.ExpiresAt == nil || claims.IssuedAt == nil || claims.Type != experimentLaunchGrantType {
		return errors.New("实验启动授权类型或有效期无效")
	}
	if claims.TenantID != tenantID || claims.AccountID != accountID || claims.ExperimentID != experimentID || claims.LessonID < 0 {
		return errors.New("实验启动授权与当前请求不匹配")
	}
	return nil
}
