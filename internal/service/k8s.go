package service

import (
	"fmt"
	"strings"
)

// Rollout 表示 Argo Rollout 的状态信息。
type Rollout struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Strategy  string `json:"strategy"`
	Status    string `json:"status"`
	Step      string `json:"step"`
	SetWeight string `json:"set_weight"`
	Ready     string `json:"ready"`
	Desired   int    `json:"desired"`
	UpToDate  int    `json:"up_to_date"`
	Available int    `json:"available"`
}

// K8sService 提供 K8s Argo Rollout 部署的业务逻辑。
type K8sService struct {
	sshManager *SSHManager
}

// NewK8sService 创建 K8s 服务实例。
func NewK8sService(sshManager *SSHManager) *K8sService {
	return &K8sService{sshManager: sshManager}
}

// ParseListOutput 解析 rollout list 脚本输出，提取 Rollout 状态信息。
func (s *K8sService) ParseListOutput(output string) []Rollout {
	var rollouts []Rollout

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 || trimmed == "" || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "NAMESPACE") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 8 {
			r := Rollout{
				Namespace: fields[0],
				Name:      fields[1],
				Strategy:  fields[2],
				Status:    fields[3],
				Step:      fields[4],
				SetWeight: fields[5],
				Ready:     fields[6],
			}
			r.Desired = parseInt(fields[7])
			if len(fields) > 8 {
				r.UpToDate = parseInt(fields[8])
			}
			if len(fields) > 9 {
				r.Available = parseInt(fields[9])
			}
			rollouts = append(rollouts, r)
		}
	}

	return rollouts
}

// GenerateSinglePreview 生成单个 Rollout 操作的命令和描述。
func (s *K8sService) GenerateSinglePreview(scriptPath, action, name, namespace string) (command, description string) {
	command = fmt.Sprintf("%s single_%s %s %s", scriptPath, action, name, namespace)

	switch action {
	case "online":
		description = fmt.Sprintf("上线 %s/%s 的 canary 版本", namespace, name)
	case "sync":
		description = fmt.Sprintf("同步 %s/%s 的全量版本", namespace, name)
	case "rollback":
		description = fmt.Sprintf("回滚 %s/%s 到上一版本", namespace, name)
	}
	return
}

// GenerateFullPreview 生成全量 Rollout 操作的命令和描述。
func (s *K8sService) GenerateFullPreview(scriptPath, action string) (command, description string) {
	command = fmt.Sprintf("%s full_%s", scriptPath, action)

	switch action {
	case "online":
		description = "全量上线所有 paused 状态的 rollout (step 1/5 → promote)"
	case "sync":
		description = "全量同步所有 paused 状态的 rollout (promote --full)"
	case "rollback":
		description = "全量回滚所有 paused 状态的 rollout"
	}
	return
}

// GenerateBatchPreview 生成批量 Rollout 操作的命令列表。
func (s *K8sService) GenerateBatchPreview(scriptPath, action string, projects []struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}) []string {
	var commands []string
	for _, p := range projects {
		if !ValidateProjectName(p.Name) || !ValidateNamespace(p.Namespace) {
			continue
		}
		cmd := fmt.Sprintf("%s single_%s %s %s", scriptPath, action, p.Name, p.Namespace)
		commands = append(commands, cmd)
	}
	return commands
}
