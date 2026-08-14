package service

import "testing"

// TestParseListOutputReadyColumn 验证 READY 列在不同资源类型下的解析：
// - Rollout/Require：Argo Rollouts 的 READY 列为单个整数（readyReplicas），
//   非 "x/y" 形式；就绪目标数应从 DESIRED 列推导。
// - Deployment：READY 列为 "x/y"，分子为就绪数，分母为目标数，且无 DESIRED 列时兜底 Current/Desired。
// 这用于修复预生产扩缩容状态误报：
// - rollout 扩容就绪后 READY=4（单整数），不应因 ReadyDesired 解析为 0 而误报"已缩容"。
// - rollout 启动中 READY=0 DESIRED=4，应显示"启动中"而非"已缩容"。
func TestParseListOutputReadyColumn(t *testing.T) {
	// Argo Rollouts 实际输出：READY 为单个整数（readyReplicas），DESIRED 为目标副本数。
	// 场景1：扩容就绪后 READY=4 DESIRED=4
	rolloutReady := `=== Rollout状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   DESIRED   CURRENT   UP-TO-DATE   AGE
svc-a                Canary     Healthy    5/5    100          4       4         4         4            1m
`
	// 场景2：启动中 READY=0 DESIRED=4（Pod 尚未就绪，控制器已设目标）
	rolloutStarting := `=== Rollout状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   DESIRED   CURRENT   UP-TO-DATE   AGE
svc-a                Canary     Healthy    5/5    100          0       4         4         4            1m
`
	// Deployment 输出：READY 为 "x/y"，无 CURRENT/DESIRED 列
	deploymentStarting := `=== Deployment状态 ===
NAME                 READY   UP-TO-DATE   AVAILABLE   AGE
svc-b                0/4     4            0           1m
`
	// Require 资源（同样是 rollout）：已就绪 READY=4 DESIRED=4
	requireReady := `=== Require列表状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   DESIRED   CURRENT   UP-TO-DATE   AGE
svc-c                Canary     Healthy    5/5    100          4       4         4         4            1m
`

	s := &PreprodService{}

	t.Run("rollout 扩容就绪 READY=4 应显示正常", func(t *testing.T) {
		res := s.ParseListOutput(rolloutReady)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		// 关键：READY 为单整数时，ReadyDesired 应从 DESIRED 列推导为 4，而非 0
		if r.Ready != 4 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 4/4", r.Ready, r.ReadyDesired)
		}
		// 不应误报为已缩容
		if r.ReadyDesired == 0 && r.Ready == 0 {
			t.Errorf("不应判定为已缩容：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
		// 应判定为就绪
		if !(r.Ready > 0 && r.Ready == r.ReadyDesired) {
			t.Errorf("应判定为就绪：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
	})

	t.Run("rollout 启动中 READY=0 DESIRED=4 应显示启动中", func(t *testing.T) {
		res := s.ParseListOutput(rolloutStarting)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		// READY=0 单整数，ReadyDesired 应从 DESIRED 推导为 4
		if r.Ready != 0 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 0/4", r.Ready, r.ReadyDesired)
		}
		// 关键：不应误报为已缩容（ready_desired=4 而非 0）
		if r.ReadyDesired == 0 && r.Ready == 0 {
			t.Errorf("不应判定为已缩容：ReadyDesired=%d 应为 4", r.ReadyDesired)
		}
		// 应处于启动中（未就绪且目标>0）
		if !(r.ReadyDesired > 0 && r.Ready < r.ReadyDesired) {
			t.Errorf("应判定为启动中：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
	})

	t.Run("deployment 启动中 READY=0/4 兜底 Current/Desired", func(t *testing.T) {
		res := s.ParseListOutput(deploymentStarting)
		if len(res) != 1 {
			t.Fatalf("期望1条资源，实际 %d", len(res))
		}
		r := res[0]
		if r.Ready != 0 || r.ReadyDesired != 4 {
			t.Errorf("Ready=%d ReadyDesired=%d，期望 0/4", r.Ready, r.ReadyDesired)
		}
		// Deployment 无 CURRENT/DESIRED 列，READY x/y 兜底 Current/Desired
		if r.Current != 0 || r.Desired != 4 {
			t.Errorf("Current=%d Desired=%d，期望 0/4（兜底）", r.Current, r.Desired)
		}
	})

	t.Run("require 已就绪 READY=4 DESIRED=4", func(t *testing.T) {
		res := s.ParseListOutput(requireReady)
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
		if !(r.Ready > 0 && r.Ready == r.ReadyDesired) {
			t.Errorf("应判定为就绪：Ready=%d ReadyDesired=%d", r.Ready, r.ReadyDesired)
		}
	})
}

// TestParseListOutputScaledDown 验证缩容状态（READY=0 DESIRED=0）的解析。
func TestParseListOutputScaledDown(t *testing.T) {
	output := `=== Rollout状态 ===
NAME                 STRATEGY   STATUS     STEP   SET-WEIGHT   READY   DESIRED   CURRENT   UP-TO-DATE   AGE
svc-a                Canary     Healthy    5/5    100          0       0         0         0            1m
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
