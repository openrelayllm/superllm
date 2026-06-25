package mailverification

import (
	"testing"

	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
	"github.com/stretchr/testify/require"
)

func TestDefaultTemplateClassifier(t *testing.T) {
	classifier := NewDefaultTemplateClassifier()

	t.Run("new api email verification", func(t *testing.T) {
		match := classifier.Classify(TemplateClassifyInput{
			SupplierType: adminplusdomain.SupplierTypeNewAPI,
			SiteName:     "Lime",
			Subject:      "Lime邮箱验证邮件",
			Text:         "您好，你正在进行Lime邮箱验证。您的验证码为: 123456 验证码 10 分钟内有效",
		})

		require.True(t, match.Matched)
		require.Equal(t, "new_api.email_verification", match.Family)
		require.Equal(t, PurposeEmailVerification, match.Purpose)
	})

	t.Run("sub2api auth verification english", func(t *testing.T) {
		match := classifier.Classify(TemplateClassifyInput{
			SupplierType: adminplusdomain.SupplierTypeSub2API,
			SiteName:     "Lime",
			Subject:      "[Lime] Email Verification Code",
			Text:         "Your verification code is: 654321",
		})

		require.True(t, match.Matched)
		require.Equal(t, "sub2api.auth_verify_code", match.Family)
		require.Equal(t, PurposeEmailVerification, match.Purpose)
	})

	t.Run("sub2api notification verification chinese", func(t *testing.T) {
		match := classifier.Classify(TemplateClassifyInput{
			SupplierType: adminplusdomain.SupplierTypeSub2API,
			SiteName:     "Lime",
			Subject:      "[Lime] 通知邮箱验证码",
			Text:         "您的验证码是：778899",
		})

		require.True(t, match.Matched)
		require.Equal(t, "sub2api.notify_verify_code", match.Family)
		require.Equal(t, PurposeNotificationEmailVerification, match.Purpose)
	})

	t.Run("password reset excluded", func(t *testing.T) {
		match := classifier.Classify(TemplateClassifyInput{
			SupplierType: adminplusdomain.SupplierTypeSub2API,
			Subject:      "[Lime] Password Reset",
			Text:         "Reset password link token 123456",
		})

		require.False(t, match.Matched)
		require.True(t, match.Excluded)
		require.Equal(t, "password_reset", match.ExcludeReason)
	})

	t.Run("site name mismatch rejects supplier template", func(t *testing.T) {
		match := classifier.Classify(TemplateClassifyInput{
			SupplierType: adminplusdomain.SupplierTypeNewAPI,
			SiteName:     "Expected",
			Subject:      "Other邮箱验证邮件",
			Text:         "您好，你正在进行Other邮箱验证。您的验证码为: 123456",
		})

		require.False(t, match.Matched)
	})
}
