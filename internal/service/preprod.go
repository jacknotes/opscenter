package service

import (
	"strings"
)

type PreprodResource struct {
	Category       string `json:"category"`
	Name           string `json:"name"`
	Desired        int    `json:"desired"`
	Current        int    `json:"current"`
	UpToDate       int    `json:"up_to_date"`
	Available      int    `json:"available"`
	Age            string `json:"age"`
	TargetReplicas int    `json:"target_replicas"`
}

type PreprodService struct {
	sshManager *SSHManager
}

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
				if !hasColumn(colRanges, "DESIRED") {
					if parts := strings.SplitN(val, "/", 2); len(parts) == 2 {
						r.Current = parseInt(parts[0])
						r.Desired = parseInt(parts[1])
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
