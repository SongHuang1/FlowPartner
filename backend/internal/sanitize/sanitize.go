package sanitize

import "regexp"

var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+\S+`),
	regexp.MustCompile(`(?i)api[_-]key\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)token\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{20,}`),
	regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)authorization[:\s]+\S+`),
}

// Error 净化错误信息，移除可能包含的敏感数据（API Key、Token、密码等）
func Error(err error) string {
	msg := err.Error()
	for _, re := range sensitivePatterns {
		if re.MatchString(msg) {
			return "API call failed (sensitive information hidden)"
		}
	}
	return msg
}
