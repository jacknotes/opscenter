package service

import "testing"

func TestValidateUpstreamName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常名称", "my-upstream", true},
		{"带下划线", "my_upstream_1", true},
		{"带数字", "upstream123", true},
		{"空字符串", "", false},
		{"含分号", "up;stream", false},
		{"含管道符", "up|stream", false},
		{"含反引号", "up`stream", false},
		{"含美元符", "up$stream", false},
		{"含空格", "up stream", false},
		{"含括号", "up(stream)", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateUpstreamName(tt.input); got != tt.want {
				t.Errorf("ValidateUpstreamName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常名称", "my-project", true},
		{"带点号", "my.project.v2", true},
		{"含分号", "proj;rm", false},
		{"含空格", "proj ect", false},
		{"含管道符", "proj|ect", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateProjectName(tt.input); got != tt.want {
				t.Errorf("ValidateProjectName(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateNamespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常命名空间", "default", true},
		{"带连字符", "my-namespace", true},
		{"含点号", "ns.test", false},
		{"含分号", "ns;rm", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateNamespace(tt.input); got != tt.want {
				t.Errorf("ValidateNamespace(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateDirectoryPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常路径", "/etc/nginx/conf.d", true},
		{"带连字符", "/opt/my-app/config", true},
		{"路径穿越", "/etc/../passwd", false},
		{"含分号", "/etc/nginx;rm -rf /", false},
		{"含管道符", "/etc/nginx|cat /etc/passwd", false},
		{"含反引号", "/etc/`whoami`", false},
		{"空字符串", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateDirectoryPath(tt.input); got != tt.want {
				t.Errorf("ValidateDirectoryPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateConfigPattern(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常模式", "*.conf", true},
		{"带连字符", "my-config.conf", true},
		{"带问号", "server?.conf", true},
		{"含分号", "*.conf;rm", false},
		{"含管道符", "*.conf|cat", false},
		{"含反引号", "*.conf`", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateConfigPattern(tt.input); got != tt.want {
				t.Errorf("ValidateConfigPattern(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"正常文件", "nginx.conf", true},
		{"带路径", "conf.d/nginx.conf", true},
		{"路径穿越", "../etc/passwd", false},
		{"含分号", "nginx.conf;rm -rf /", false},
		{"空字符串", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateFilePath(tt.input); got != tt.want {
				t.Errorf("ValidateFilePath(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
