package agent

import "CompeteAI/internal/domain"

// AgentCard Agent 自描述（§6）。
type AgentCard struct {
	Name            domain.AgentName `json:"name"`
	DisplayName     string           `json:"displayName"`
	Description     string           `json:"description"`
	Skills          []string         `json:"skills"`
	Tools           []string         `json:"tools"`
	DependsOn       []string         `json:"dependsOn"`
	InputArtifacts  []string         `json:"inputArtifacts,omitempty"`
	OutputArtifacts []string         `json:"outputArtifacts,omitempty"`
}

// WebAgentCard 供 HTTP 返回（name 为 string）。
type WebAgentCard struct {
	Name            string   `json:"name"`
	DisplayName     string   `json:"displayName"`
	Description     string   `json:"description"`
	Skills          []string `json:"skills"`
	Tools           []string `json:"tools"`
	DependsOn       []string `json:"dependsOn"`
	InputArtifacts  []string `json:"inputArtifacts,omitempty"`
	OutputArtifacts []string `json:"outputArtifacts,omitempty"`
}

func (c AgentCard) ToWeb() WebAgentCard {
	return WebAgentCard{
		Name:            string(c.Name),
		DisplayName:     c.DisplayName,
		Description:     c.Description,
		Skills:          c.Skills,
		Tools:           c.Tools,
		DependsOn:       c.DependsOn,
		InputArtifacts:  c.InputArtifacts,
		OutputArtifacts: c.OutputArtifacts,
	}
}
