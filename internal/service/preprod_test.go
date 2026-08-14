package service

import "testing"

// TestParseListOutputReadyColumn 验证 READY 列 (x/y) 在不同资源类型下的解析：
// - Rollout/Require：同时有 CURRENT/DESIRED 与 READY 列，Ready/ReadyDesired 取自 READY，
//   Current/Desired 取自各自列（不被 READY 覆盖）。
// - Deployment：无 CURRENT/DESIRED 列，READY 兜底 Current/Desired，同时填充 Ready/ReadyDesired。
// 这用于修复预生产扩缩容"已正常"状态误报：脚本等 Pod 就绪时不应显示"正常"。
func TestParseListOutputReadyColumn(t *testing.T) {
	// 模拟 kubectl get rollout 输出：READY=0/4（Pod 未就绪），CURRENT=4, DESIRED=4
	rolloutOutput := `=== Rollout状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   CURRENT   DESIRED   UP-TO-DATE   AGE
svc-a                Canary     Healthy    5/5    100          0/4     4         4         4            1m
`
	// 模拟 kubectl get deployment 输出：READY=0/4（Pod 启动中）
	deploymentOutput := `=== Deployment状态 ===
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
svc-b                0/4     4            0           1m
`
	// 模拟 kubectl get rollout 输出（require 资源）：READY=4/4（已就绪）
	requireOutput := `=== Require列表状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   CURRENT   DESIRED   UP-TO-DATE   AGE
svc-c                Canary     Healthy    5/5    100          4/4     4         4         4            1m
`

	s := &PreprodService{}

	t.Run("rollout 启动中 READY=0/4 应解析就绪为0", func(t *testing.T) {
		res := s.ParseListOutput(rolloutOutput)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		if r.Ready != 0 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 0/4", r.Ready, r.ReadyDesired)
		}
		// Current/Desired 应取自 CURRENT/DESIRED 列，不被 READY 覆盖
		if r.Current != 4 || r.Desired != 4 {
			t.Errorf("Current=%d Desired=%d，期望 4/4", r.Current, r.Desired)
		}
		// 关键：就绪数 0 < 目标 4，不应判定为"正常"
		if r.Ready > 0 && r.Ready == r.ReadyDesired {
			t.Errorf("不应判定为就绪：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
	})

	t.Run("deployment 启动中 READY=0/4 兜底 Current/Desired", func(t *testing.T) {
		res := s.ParseListOutput(deploymentOutput)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		if r.Ready != 0 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 0/4", r.Ready, r.ReadyDesired)
		}
		// Deployment 无 CURRENT/DESIRED 列，READY 兜底 Current/Desired
		if r.Current != 0 || r.Desired != 4 {
			t.Errorf("Current=%d Desired=%d，期望 0/4（兜底）", r.Current, r.Desired)
		}
	})

	t.Run("require 已就绪 READY=4/4", func(t *testing.T) {
		res := s.ParseListOutput(requireOutput)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		if r.Category != "require" {
			t.Errorf("Category=%s，期望 require", r.Category)
		}
		if r.Ready != 4 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 4/4", r.Ready, r.ReadyDesired)
		}
		// 已就绪：Ready>0 且 Ready==ReadyDesired
		if !(r.Ready > 0 && r.Ready == r.ReadyDesired) {
			t.Errorf("应判定为就绪：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
	})
}

// TestParseListOutputScaledDown 验证缩容状态（READY=0/0）的解析。
func TestParseListOutputScaledDown(t *testing.T) {
	output := `=== Rollout状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   CURRENT   DESIRED   UP-TO-DATE   AGE
svc-a                Canary     Healthy    5/5    100          0/0     0         0         0            1m
`
	s := &PreprodService{}
	res := s.ParseListOutput(output)
	if len(res) != 1 {
		t.Fatalf("期望1条资源，实际 %d", len(res))
	}
	r := res[0]
	if r.Ready != 0 || r.ReadyDesired != 0 {
		t.Errorf("Ready=%d ReadyDesired=%d，期望 0/0", r.Ready, r.ReadyDesired)
	}
	// 缩容状态：ready==0 且 ready_desired==0
	if !(r.Ready == 0 && r.ReadyDesired == 0) {
		t.Errorf("应判定为已缩容：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
	}
}
