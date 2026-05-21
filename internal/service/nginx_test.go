package service

import "testing"

func TestGenerateModifyCommand_UpstreamInjection(t *testing.T) {
	svc := &NginxService{}

	// 正常 upstream name 应该生成命令
	cmd := svc.GenerateModifyCommand("/etc/nginx", "nginx.conf", "my-upstream", []string{"10.0.0.1"}, "online")
	if cmd == "" {
		t.Error("正常 upstream name 不应返回空命令")
	}

	// 注入攻击应返回空命令
	cmd = svc.GenerateModifyCommand("/etc/nginx", "nginx.conf", "upstream;rm -rf /", []string{"10.0.0.1"}, "online")
	if cmd != "" {
		t.Errorf("注入 upstream name 应返回空命令，实际返回: %s", cmd)
	}

	cmd = svc.GenerateModifyCommand("/etc/nginx", "nginx.conf", "up`whoami`", []string{"10.0.0.1"}, "online")
	if cmd != "" {
		t.Errorf("反引号注入应返回空命令，实际返回: %s", cmd)
	}
}

func TestGenerateSwapModifyCommands_UpstreamInjection(t *testing.T) {
	svc := &NginxService{}

	// 正常 upstream name
	cmds := svc.GenerateSwapModifyCommands("/etc/nginx", "nginx.conf", "my-upstream", "10.0.0.1:80", "10.0.0.2:80")
	if len(cmds) == 0 {
		t.Error("正常 upstream name 不应返回空命令列表")
	}

	// 注入攻击
	cmds = svc.GenerateSwapModifyCommands("/etc/nginx", "nginx.conf", "up;rm", "10.0.0.1:80", "10.0.0.2:80")
	if len(cmds) != 0 {
		t.Errorf("注入 upstream name 应返回空命令列表，实际返回 %d 条", len(cmds))
	}
}

func TestGenerateToggleModifyCommands_UpstreamInjection(t *testing.T) {
	svc := &NginxService{}
	servers := []NginxServer{{IP: "10.0.0.1", Port: "80", Status: "up"}}

	// 正常 upstream name
	cmds := svc.GenerateToggleModifyCommands("/etc/nginx", "nginx.conf", "my-upstream", servers)
	if len(cmds) == 0 {
		t.Error("正常 upstream name 不应返回空命令列表")
	}

	// 注入攻击
	cmds = svc.GenerateToggleModifyCommands("/etc/nginx", "nginx.conf", "up$()", servers)
	if len(cmds) != 0 {
		t.Errorf("注入 upstream name 应返回空命令列表，实际返回 %d 条", len(cmds))
	}
}

func TestParseConfig(t *testing.T) {
	svc := &NginxService{}
	config := `
upstream backend {
    server 10.0.0.1:8080 weight=1;
    server 10.0.0.2:8080 weight=2;
    #server 10.0.0.3:8080;
}
`
	upstreams := svc.ParseConfig(config)
	if len(upstreams) != 1 {
		t.Fatalf("期望 1 个 upstream，实际 %d", len(upstreams))
	}
	if upstreams[0].Name != "backend" {
		t.Errorf("期望 upstream 名称为 backend，实际 %s", upstreams[0].Name)
	}
	if len(upstreams[0].Servers) != 3 {
		t.Errorf("期望 3 个 server，实际 %d", len(upstreams[0].Servers))
	}
	// 检查注释的 server 状态为 down
	for _, srv := range upstreams[0].Servers {
		if srv.IP == "10.0.0.3" && srv.Status != "down" {
			t.Error("注释的 server 状态应为 down")
		}
	}
}

func TestGenerateDiff_Online(t *testing.T) {
	svc := &NginxService{}
	config := `upstream backend {
    server 10.0.0.1:8080;
    #server 10.0.0.2:8080;
}`

	before, after := svc.GenerateDiff(config, "backend", "10.0.0.2:8080", "online")
	if before != config {
		t.Error("before 应与原配置相同")
	}
	if after == config {
		t.Error("online 操作后配置应有变化")
	}
	// online 操作应该去掉注释
	if contains(after, "#server 10.0.0.2") {
		t.Error("online 操作后不应有注释的 server")
	}
}

func TestGenerateDiff_Offline(t *testing.T) {
	svc := &NginxService{}
	config := `upstream backend {
    server 10.0.0.1:8080;
    server 10.0.0.2:8080;
}`

	_, after := svc.GenerateDiff(config, "backend", "10.0.0.2:8080", "offline")
	if !contains(after, "#server 10.0.0.2") {
		t.Error("offline 操作后应有注释的 server")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
