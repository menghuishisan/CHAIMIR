// privacy 提供跨模块可复用的用户数据脱敏与敏感字段识别函数。
package privacy

import "strings"

var credentialKeyMarkers = []string{
	"password",
	"passwd",
	"private_key",
	"privatekey",
	"access_key",
	"accesskey",
	"signing_key",
	"signingkey",
	"session_secret",
	"sessionsecret",
	"secret",
	"token",
	"credential",
	"authorization",
	"api_key",
	"apikey",
}

var resultSensitiveKeyMarkers = append(append([]string{}, credentialKeyMarkers...),
	"answer",
	"answers",
	"correct_answer",
	"solution",
	"judge_config",
	"testcases",
	"hidden_testcases",
	"flag",
	"flags",
	"answer_source",
	"suite_source",
)

// MaskPhone 对中国大陆手机号做用户向掩码展示,非法长度返回空字符串避免误展示原值。
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) != 11 {
		return ""
	}
	return phone[:3] + "****" + phone[7:]
}

// IsCredentialKey 判断字段名是否携带密码、token、密钥等凭据语义。
func IsCredentialKey(key string) bool {
	return containsKeyMarker(key, credentialKeyMarkers)
}

// IsResultSensitiveKey 判断用户可见结果字段是否可能携带答案、flag 或凭据。
func IsResultSensitiveKey(key string) bool {
	return containsKeyMarker(key, resultSensitiveKeyMarkers)
}

// ContainsResultSensitiveText 判断用户可见文本中是否包含明显答案、flag 或私钥片段。
func ContainsResultSensitiveText(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"flag{", "-----begin", "answer_source", "suite_source"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return IsResultSensitiveKey(lower)
}

// containsKeyMarker 按字段名包含关系判断敏感语义。
func containsKeyMarker(key string, markers []string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
