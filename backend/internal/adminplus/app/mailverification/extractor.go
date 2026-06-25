package mailverification

import (
	"html"
	"regexp"
	"strings"
)

var (
	chineseCodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:验证码|校验码|动态码)[^a-z0-9]{0,24}([a-z0-9]{4,10})`),
		regexp.MustCompile(`(?i)(?:您的验证码为|您的验证码是|验证码为|验证码是)[^a-z0-9]{0,12}([a-z0-9]{4,10})`),
	}
	englishCodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:verification code|security code|login code|auth code)[^a-z0-9]{0,36}([a-z0-9]{4,10})`),
		regexp.MustCompile(`(?i)(?:code is|code:)[^a-z0-9]{0,18}([a-z0-9]{4,10})`),
	}
	genericSixDigitPattern = regexp.MustCompile(`\b(\d{6})\b`)
	htmlTagPattern         = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlScriptStylePattern = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>|<style[^>]*>.*?</style>`)
)

func ExtractVerificationCode(text string) string {
	text = normalizeText(text)
	if text == "" {
		return ""
	}
	for _, pattern := range chineseCodePatterns {
		if code := findCode(text, pattern); code != "" {
			return code
		}
	}
	for _, pattern := range englishCodePatterns {
		if code := findCode(text, pattern); code != "" {
			return code
		}
	}
	return findCode(text, genericSixDigitPattern)
}

func findCode(text string, pattern *regexp.Regexp) string {
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	code := strings.TrimSpace(match[1])
	if len(code) < 4 || len(code) > 10 {
		return ""
	}
	if !strings.ContainsAny(code, "0123456789") {
		return ""
	}
	return code
}

func NormalizeMailBody(text string) string {
	return normalizeText(stripHTML(text))
}

func stripHTML(text string) string {
	text = htmlScriptStylePattern.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "<br>", "\n")
	text = strings.ReplaceAll(text, "<br/>", "\n")
	text = strings.ReplaceAll(text, "<br />", "\n")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	return html.UnescapeString(text)
}

func normalizeText(text string) string {
	text = strings.ReplaceAll(text, "\u00a0", " ")
	text = strings.ReplaceAll(text, "\r", "\n")
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}
