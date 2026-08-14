package service

import (
	"strings"
)

// PreprodResource 表示预生产环境中的可伸缩资源（Rollout/Deployment/StatefulSet）。
type PreprodResource struct {
	Category       string `json:"category"`
	Name           string `json:"name"`
	Desired        int    `json:"desired"`
	Current        int    `json:"current"`
	UpToDate       int    `json:"up_to_date"`
	Available      int    `json:"available"`
	Age            string `json:"age"`
	TargetReplicas int    `json:"target_replicas"`
	// Ready 为 READY 列的就绪副本数（分子），ReadyDesired 为 READY 列的目标副本数（分母）。
	// 与控制器视角的 Current/Desired 不同，二者反映 Pod 真实就绪状态，
	// 用于准确判断扩容过程中 Pod 是否真正 Running。
	Ready        int `json:"ready"`
	ReadyDesired int `json:"ready_desired"`
}

// PreprodService 提供预生产缩扩容的业务逻辑。
type PreprodService struct {
	sshManager *SSHManager
}

// NewPreprodService 创建预生产服务实例。
func NewPreprodService(sshManager *SSHManager) *PreprodService {
	return &PreprodService{sshManager: sshManager}
}

type columnRange struct {
	Name  string
	Start int
	End   int
}

func buildColumnRanges(headerLine string, fields []string) []columnRange {
	n := len(fields)
	starts := make([]int, n)
	pos := 0
	for i, f := range fields {
		idx := strings.Index(headerLine[pos:], f)
		if idx < 0 {
			continue
		}
		starts[i] = pos + idx
		pos = starts[i] + len(f)
	}

	ranges := make([]columnRange, n)
	for i := 0; i < n; i++ {
		end := 1 << 30
		if i+1 < n {
			end = starts[i+1]
		}
		ranges[i] = columnRange{
			Name:  strings.ToUpper(fields[i]),
			Start: starts[i],
			End:   end,
		}
	}
	return ranges
}

func extractColumn(line string, cr columnRange) string {
	if cr.Start >= len(line) {
		return ""
	}
	end := cr.End
	if end > len(line) {
		end = len(line)
	}
	return strings.TrimSpace(line[cr.Start:end])
}

func hasColumn(ranges []columnRange, name string) bool {
	for _, cr := range ranges {
		if cr.Name == name {
			return true
		}
	}
	return false
}

// findColumn 按名称查找列范围，未找到返回 nil。
func findColumn(ranges []columnRange, name string) *columnRange {
	for i := range ranges {
		if ranges[i].Name == name {
			return &ranges[i]
		}
	}
	return nil
}

// ParseListOutput 解析 list 脚本输出，按类别（rollout/deployment/statefulset）提取资源状态。
// 使用列位置解析而非固定字段分割，兼容不同列宽的输出格式。
func (s *PreprodService) ParseListOutput(output string) []PreprodResource {
	var resources []PreprodResource
	currentCategory := ""
	var colRanges []columnRange

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "===") {
			if strings.Contains(trimmed, "Rollout") {
				currentCategory = "rollout"
			} else if strings.Contains(trimmed, "Deployment") {
				currentCategory = "deployment"
			} else if strings.Contains(trimmed, "StatefulSet") {
				currentCategory = "statefulset"
			} else if strings.Contains(trimmed, "Require") {
				currentCategory = "require"
			}
			colRanges = nil
			continue
		}

		if strings.HasPrefix(trimmed, "-") || trimmed == "" {
			continue
		}

		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}

		if fields[0] == "NAME" {
			colRanges = buildColumnRanges(line, fields)
			continue
		}

		if currentCategory == "" || len(colRanges) == 0 {
			continue
		}

		r := PreprodResource{Category: currentCategory}
		for _, cr := range colRanges {
			val := extractColumn(line, cr)
			switch cr.Name {
			case "NAME":
				r.Name = val
			case "DESIRED":
				r.Desired = parseInt(val)
			case "CURRENT":
				r.Current = parseInt(val)
			case "UP-TO-DATE":
				r.UpToDate = parseInt(val)
			case "AVAILABLE":
				r.Available = parseInt(val)
			case "AGE":
				r.Age = val
			case "READY":
				// READY 列可能为 "x/y"（Deployment）或单个整数（Argo Rollouts 的 READY 列
				// 仅打印 readyReplicas）。统一解析为就绪副本数 Ready 与目标就绪数 ReadyDesired，
				// 用于准确判断扩容过程中 Pod 是否真正 Running。
				if parts := strings.SplitN(val, "/", 2); len(parts) == 2 {
					// "x/y" 形式：分子为就绪数，分母为目标数
					r.Ready = parseInt(parts[0])
					r.ReadyDesired = parseInt(parts[1])
					// 仅当无 DESIRED 列时（如 Deployment），用 READY 兜底 Current/Desired。
					if !hasColumn(colRanges, "DESIRED") {
						r.Current = r.Ready
						r.Desired = r.ReadyDesired
					}
				} else {
					// 单个整数形式（Argo Rollouts）：值为就绪数，目标数取自 DESIRED 列
					r.Ready = parseInt(val)
					if cr := findColumn(colRanges, "DESIRED"); cr != nil {
						if desiredVal := extractColumn(line, *cr); desiredVal != "" {
							r.ReadyDesired = parseInt(desiredVal)
						}
					}
				}
			}
		}
		resources = append(resources, r)
	}
	return resources
}

type TargetInfo struct {
	Category string
	Name     string
	Replicas int
}

// ParseTargetOutput 解析 list-targets 脚本输出，提取每个资源的目标副本数。
func (s *PreprodService) ParseTargetOutput(output string) []TargetInfo {
	var targets []TargetInfo
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 4 {
			continue
		}
		targets = append(targets, TargetInfo{
			Category: fields[0],
			Name:     fields[2],
			Replicas: parseInt(fields[3]),
		})
	}
	return targets
}

// MergeTargets 将目标副本数合并到资源列表中，用于展示扩容时的目标值。
func (s *PreprodService) MergeTargets(resources []PreprodResource, targets []TargetInfo) []PreprodResource {
	targetMap := make(map[string]int)
	for _, t := range targets {
		targetMap[t.Name] = t.Replicas
	}
	for i := range resources {
		if rep, ok := targetMap[resources[i].Name]; ok {
			resources[i].TargetReplicas = rep
		}
	}
	return resources
}

// GeneratePreview 生成缩扩容操作的命令和描述。resourceNames 为空时操作所有资源。
func (s *PreprodService) GeneratePreview(scriptPath, action string, resourceNames []string) (command, description string) {
	command = scriptPath + " " + action
	if len(resourceNames) > 0 {
		command += " " + strings.Join(resourceNames, " ")
	}

	switch action {
	case "scaledown":
		if len(resourceNames) > 0 {
			description = "缩容选中的 " + strings.Join(resourceNames, ", ") + " 副本数至 0"
		} else {
			description = "缩容所有资源副本数至 0"
		}
	case "scaleup":
		if len(resourceNames) > 0 {
			description = "扩容选中的 " + strings.Join(resourceNames, ", ") + " 至目标副本数"
		} else {
			description = "扩容所有资源至目标副本数"
		}
	}
	return
}
