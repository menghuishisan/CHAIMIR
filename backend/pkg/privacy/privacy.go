// Package privacy 提供数据脱敏和敏感字段识别。
package privacy

import "strings"

var keySeparators = strings.NewReplacer("-", "", "_", "")

var credentialKeyMarkers = []string{
	"password",
	"passwd",
	"privatekey",
	"accesskey",
	"signingkey",
	"sessionsecret",
	"secret",
	"token",
	"credential",
	"authorization",
	"apikey",
	"cookie",
}

var resultSensitiveKeyMarkers = []string{
	"answer",
	"solution",
	"judgeconfig",
	"testcases",
	"flag",
	"suitesource",
}

// MaskPhone 对中国大陆手机号做用户向掩码展示,输入须为 11 位数字。
func MaskPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if len(phone) != 11 {
		return ""
	}
	for i := range len(phone) {
		if phone[i] < '0' || phone[i] > '9' {
			return ""
		}
	}
	return phone[:3] + "****" + phone[7:]
}

// IsCredentialKey 判断字段名是否携带密码、token、密钥等凭据语义。
func IsCredentialKey(key string) bool {
	return containsKeyMarker(key, credentialKeyMarkers)
}

// IsResultSensitiveKey 判断用户可见结果字段是否可能携带答案、flag 或凭据。
func IsResultSensitiveKey(key string) bool {
	return IsCredentialKey(key) || containsKeyMarker(key, resultSensitiveKeyMarkers)
}

// ContainsResultSensitiveText 判断用户可见文本中是否包含明显答案、flag 或私钥片段。
func ContainsResultSensitiveText(value string) bool {
	return strings.Contains(strings.ToLower(value), "-----begin") || IsResultSensitiveKey(value)
}

func containsKeyMarker(key string, markers []string) bool {
	normalized := keySeparators.Replace(strings.ToLower(strings.TrimSpace(key)))
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}
