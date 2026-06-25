package mailverification

import (
	"strings"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

type DefaultTemplateClassifier struct{}

func NewDefaultTemplateClassifier() *DefaultTemplateClassifier {
	return &DefaultTemplateClassifier{}
}

func (c *DefaultTemplateClassifier) Classify(in TemplateClassifyInput) adminplusdomain.MailTemplateMatch {
	supplierType := adminplusdomain.NormalizeSupplierType(string(in.SupplierType))
	expectedPurpose := normalizePurpose(in.ExpectedPurpose)
	subject := strings.TrimSpace(in.Subject)
	body := strings.TrimSpace(in.Text + "\n" + in.Snippet)
	allLower := strings.ToLower(subject + "\n" + body)

	if isPasswordResetMail(allLower) {
		return adminplusdomain.MailTemplateMatch{
			SupplierType:  supplierType,
			Purpose:       expectedPurpose,
			Excluded:      true,
			ExcludeReason: "password_reset",
		}
	}

	switch supplierType {
	case adminplusdomain.SupplierTypeNewAPI:
		return classifyNewAPI(subject, body, expectedPurpose, in.SiteName)
	case adminplusdomain.SupplierTypeSub2API:
		return classifySub2API(subject, body, expectedPurpose, in.SiteName)
	case "":
		if ExtractVerificationCode(subject+"\n"+body) != "" {
			return adminplusdomain.MailTemplateMatch{
				Purpose:    firstNonEmpty(expectedPurpose, PurposeEmailVerification),
				Family:     "generic.verification_code",
				Confidence: 0.55,
				Matched:    true,
			}
		}
	}

	return adminplusdomain.MailTemplateMatch{
		SupplierType: supplierType,
		Purpose:      expectedPurpose,
	}
}

func classifyNewAPI(subject, body, expectedPurpose, siteName string) adminplusdomain.MailTemplateMatch {
	all := subject + "\n" + body
	hasSubject := strings.Contains(subject, "邮箱验证邮件")
	hasBody := strings.Contains(all, "邮箱验证") &&
		(strings.Contains(all, "您的验证码为") || strings.Contains(all, "验证码为"))
	if !hasSubject || !hasBody || !subjectMatchesSite(subject, siteName) || ExtractVerificationCode(all) == "" {
		return adminplusdomain.MailTemplateMatch{
			SupplierType: adminplusdomain.SupplierTypeNewAPI,
			Purpose:      expectedPurpose,
		}
	}
	purpose := PurposeEmailVerification
	if expectedPurpose != "" && expectedPurpose != purpose {
		return adminplusdomain.MailTemplateMatch{
			SupplierType: adminplusdomain.SupplierTypeNewAPI,
			Purpose:      expectedPurpose,
		}
	}
	return adminplusdomain.MailTemplateMatch{
		SupplierType: adminplusdomain.SupplierTypeNewAPI,
		Purpose:      purpose,
		Family:       "new_api.email_verification",
		Confidence:   0.95,
		Matched:      true,
	}
}

func classifySub2API(subject, body, expectedPurpose, siteName string) adminplusdomain.MailTemplateMatch {
	all := subject + "\n" + body
	allLower := strings.ToLower(all)
	subjectLower := strings.ToLower(subject)
	isNotification := strings.Contains(subjectLower, "notification email verification") ||
		strings.Contains(subject, "通知邮箱验证码")
	hasSubject := strings.Contains(subjectLower, "email verification code") ||
		strings.Contains(subjectLower, "email verification") ||
		strings.Contains(subject, "邮箱验证码") ||
		isNotification
	hasBody := strings.Contains(allLower, "your verification code is") ||
		strings.Contains(all, "您的验证码是")
	if !hasSubject || !hasBody || !subjectMatchesSite(subject, siteName) || ExtractVerificationCode(all) == "" {
		return adminplusdomain.MailTemplateMatch{
			SupplierType: adminplusdomain.SupplierTypeSub2API,
			Purpose:      expectedPurpose,
		}
	}
	purpose := PurposeEmailVerification
	family := "sub2api.auth_verify_code"
	if isNotification {
		purpose = PurposeNotificationEmailVerification
		family = "sub2api.notify_verify_code"
	}
	if expectedPurpose != "" && expectedPurpose != purpose {
		return adminplusdomain.MailTemplateMatch{
			SupplierType: adminplusdomain.SupplierTypeSub2API,
			Purpose:      expectedPurpose,
		}
	}
	return adminplusdomain.MailTemplateMatch{
		SupplierType: adminplusdomain.SupplierTypeSub2API,
		Purpose:      purpose,
		Family:       family,
		Confidence:   0.95,
		Matched:      true,
	}
}

func subjectMatchesSite(subject, siteName string) bool {
	siteName = strings.TrimSpace(siteName)
	if siteName == "" {
		return true
	}
	return strings.Contains(strings.ToLower(subject), strings.ToLower(siteName))
}

func isPasswordResetMail(text string) bool {
	resetTerms := []string{
		"password reset",
		"reset password",
		"forgot password",
		"重置密码",
		"找回密码",
		"密码重置",
	}
	for _, term := range resetTerms {
		if strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func normalizePurpose(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case PurposeEmailVerification, PurposeNotificationEmailVerification:
		return value
	default:
		return value
	}
}
