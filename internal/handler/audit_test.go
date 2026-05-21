package handler

import "testing"

func TestSanitizeCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"password关键字",
			"mysql -u root -pMySecret123 -h localhost",
			"mysql -u root *** -h localhost",
		},
		{
			"password等号",
			"connect --password=secret123 --host=localhost",
			"connect --password=*** --host=localhost",
		},
		{
			"echo base64密码",
			"echo 'U2VjcmV0UGFzc3dvcmQ=' | base64 -d | sudo -S command",
			"*** -d | sudo -S command",
		},
		{
			"echo sudo密码",
			`echo "MyPassword" | sudo -S apt-get install`,
			"***  apt-get install",
		},
		{
			"无敏感信息",
			"nginx -t && systemctl reload nginx",
			"nginx -t && systemctl reload nginx",
		},
		{
			"空字符串",
			"",
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeCommand(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeCommand(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
