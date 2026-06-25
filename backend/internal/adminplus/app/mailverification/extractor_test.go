package mailverification

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractVerificationCode(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "new api chinese pattern",
			text: "您好，你正在进行 Lime 邮箱验证。您的验证码为: 123456，验证码 10 分钟内有效",
			want: "123456",
		},
		{
			name: "sub2api english pattern",
			text: "Your verification code is: 654321. It expires in 15 minutes.",
			want: "654321",
		},
		{
			name: "generic six digits fallback",
			text: "Use 112233 to continue.",
			want: "112233",
		},
		{
			name: "chinese alphanumeric code",
			text: "您好，你正在进行示例站点邮箱验证。您的验证码为: b39bbd 验证码 10 分钟内有效",
			want: "b39bbd",
		},
		{
			name: "empty",
			text: "no code here",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ExtractVerificationCode(tt.text))
		})
	}
}

func TestNormalizeMailBodyStripsHTML(t *testing.T) {
	text := NormalizeMailBody(`<html><style>.x{}</style><body><p>您的验证码是：<strong>778899</strong></p></body></html>`)
	require.Contains(t, text, "您的验证码是： 778899")
	require.Equal(t, "778899", ExtractVerificationCode(text))
}
