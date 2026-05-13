package service

import (
	"strings"
)

type PreprodResource struct {
	Category    string `json:"category"`
	Name        string `json:"name"`
	Desired     int    `json:"desired"`
	Current     int    `json:"current"`
	UpToDate    int    `json:"up_to_date"`
	Available   int    `json:"available"`
	Age         string `json:"age"`
}

type PreprodService struct {
	sshManager *SSHManager
}

func NewPreprodService(sshManager *SSHManager) *PreprodService {
	return &PreprodService{sshManager: sshManager}
}

func (s *PreprodService) ParseListOutput(output string) []PreprodResource {
	var resources []PreprodResource
	currentCategory := ""

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "===") {
			if strings.Contains(line, "Rollout") {
				currentCategory = "rollout"
			} else if strings.Contains(line, "Deployment") {
				currentCategory = "deployment"
			} else if strings.Contains(line, "Require") {
				currentCategory = "require"
			}
			continue
		}

		if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "NAME") || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			r := PreprodResource{
				Category: currentCategory,
				Name:     fields[0],
			}
			r.Desired = parseInt(fields[1])
			r.Current = parseInt(fields[2])
			r.UpToDate = parseInt(fields[3])
			r.Available = parseInt(fields[4])
			if len(fields) > 5 {
				r.Age = fields[5]
			}
			resources = append(resources, r)
		}
	}

	return resources
}

func (s *PreprodService) GeneratePreview(scriptPath, action string) (command, description string) {
	command = scriptPath + " " + action

	switch action {
	case "scaledown":
		description = "缩容所有资源副本数至 0"
	case "scaleup":
		description = "扩容所有资源至目标副本数"
	}
	return
}
