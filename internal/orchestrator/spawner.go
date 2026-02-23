// Package orchestrator handles building agent pods and spawning sub-agents.
package orchestrator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sclawv1alpha1 "github.com/k8sclaw/k8sclaw/api/v1alpha1"
)

// Spawner handles sub-agent spawn requests by creating AgentRun CRs.
type Spawner struct {
	Client client.Client
	Log    logr.Logger
}

// SpawnRequest represents a request from a parent agent to spawn a sub-agent.
type SpawnRequest struct {
	// ParentRunName is the name of the parent AgentRun.
	ParentRunName string `json:"parentRunName"`

	// ParentSessionKey is the session key of the parent.
	ParentSessionKey string `json:"parentSessionKey"`

	// InstanceName is the ClawInstance this belongs to.
	InstanceName string `json:"instanceName"`

	// Namespace is the Kubernetes namespace.
	Namespace string `json:"namespace"`

	// Task is the task for the sub-agent.
	Task string `json:"task"`

	// SystemPrompt is the system prompt for the sub-agent.
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// AgentID is the agent configuration to use.
	AgentID string `json:"agentId"`

	// CurrentDepth is the current spawn depth.
	CurrentDepth int `json:"currentDepth"`

	// Model configuration.
	Model k8sclawv1alpha1.ModelSpec `json:"model"`

	// Skills to mount.
	Skills []k8sclawv1alpha1.SkillRef `json:"skills,omitempty"`
}

// SpawnResult is the result of a spawn operation.
type SpawnResult struct {
	// RunName is the name of the created AgentRun.
	RunName string `json:"runName"`

	// Error is set if the spawn failed.
	Error string `json:"error,omitempty"`
}

// Spawn creates a new AgentRun CR for a sub-agent.
func (s *Spawner) Spawn(ctx context.Context, req SpawnRequest) (*SpawnResult, error) {
	log := s.Log.WithValues(
		"parentRun", req.ParentRunName,
		"instance", req.InstanceName,
		"depth", req.CurrentDepth+1,
	)

	runName := fmt.Sprintf("sub-%s-%d", req.ParentRunName, req.CurrentDepth+1)
	sessionKey := fmt.Sprintf("%s:sub:%s", req.ParentSessionKey, runName)

	log.Info("Spawning sub-agent", "runName", runName)

	agentRun := &k8sclawv1alpha1.AgentRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runName,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"k8sclaw.io/instance":   req.InstanceName,
				"k8sclaw.io/agent-id":   req.AgentID,
				"k8sclaw.io/parent-run": req.ParentRunName,
				"k8sclaw.io/component":  "agent-run",
			},
		},
		Spec: k8sclawv1alpha1.AgentRunSpec{
			InstanceRef: req.InstanceName,
			AgentID:     req.AgentID,
			SessionKey:  sessionKey,
			Parent: &k8sclawv1alpha1.ParentRunRef{
				RunName:    req.ParentRunName,
				SessionKey: req.ParentSessionKey,
				SpawnDepth: req.CurrentDepth + 1,
			},
			Task:         req.Task,
			SystemPrompt: req.SystemPrompt,
			Model:        req.Model,
			Skills:       req.Skills,
			Cleanup:      "delete",
		},
	}

	if err := s.Client.Create(ctx, agentRun); err != nil {
		return &SpawnResult{Error: err.Error()}, err
	}

	return &SpawnResult{RunName: runName}, nil
}
