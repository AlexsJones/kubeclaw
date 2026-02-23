// Package webhook provides validating and mutating admission webhooks for KubeClaw.
// These enforce ClawPolicy constraints on AgentRun resources.
package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	kubeclawv1alpha1 "github.com/kubeclaw/kubeclaw/api/v1alpha1"
)

// PolicyEnforcer is a validating webhook that enforces ClawPolicy on AgentRuns.
type PolicyEnforcer struct {
	Client  client.Client
	Log     logr.Logger
	decoder admission.Decoder
}

// Handle validates AgentRun creation/updates against the bound ClawPolicy.
func (pe *PolicyEnforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	run := &kubeclawv1alpha1.AgentRun{}
	if err := pe.decoder.Decode(req, run); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Look up the owning ClawInstance
	var instance kubeclawv1alpha1.ClawInstance
	if err := pe.Client.Get(ctx, types.NamespacedName{
		Name:      run.Spec.InstanceRef,
		Namespace: run.Namespace,
	}, &instance); err != nil {
		return admission.Errored(http.StatusBadRequest,
			fmt.Errorf("failed to find ClawInstance %s: %w", run.Spec.InstanceRef, err))
	}

	// If no policy is bound, allow
	if instance.Spec.PolicyRef == "" {
		return admission.Allowed("no policy bound")
	}

	// Look up the ClawPolicy
	var policy kubeclawv1alpha1.ClawPolicy
	if err := pe.Client.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.PolicyRef,
		Namespace: run.Namespace,
	}, &policy); err != nil {
		return admission.Errored(http.StatusInternalServerError,
			fmt.Errorf("failed to find ClawPolicy %s: %w", instance.Spec.PolicyRef, err))
	}

	// Validate sandbox policy
	if policy.Spec.SandboxPolicy != nil && policy.Spec.SandboxPolicy.Required {
		if run.Spec.Sandbox == nil || !run.Spec.Sandbox.Enabled {
			return admission.Denied("sandbox is required by policy")
		}
	}

	// Validate resource limits
	if err := pe.validateResources(run, &policy); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate sub-agent depth
	if err := pe.validateSubagentDepth(run, &policy); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate tool policy
	if err := pe.validateToolPolicy(run, &policy); err != nil {
		return admission.Denied(err.Error())
	}

	// Validate feature gates
	if err := pe.validateFeatureGates(run, &policy); err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("policy validated")
}

func (pe *PolicyEnforcer) validateResources(run *kubeclawv1alpha1.AgentRun, policy *kubeclawv1alpha1.ClawPolicy) error {
	if policy.Spec.SandboxPolicy == nil || run.Spec.Sandbox == nil {
		return nil
	}

	if policy.Spec.SandboxPolicy.MaxCPU != "" {
		maxCPU := resource.MustParse(policy.Spec.SandboxPolicy.MaxCPU)
		_ = maxCPU // Would compare against run's resource requests
	}

	if policy.Spec.SandboxPolicy.MaxMemory != "" {
		maxMem := resource.MustParse(policy.Spec.SandboxPolicy.MaxMemory)
		_ = maxMem
	}

	return nil
}

func (pe *PolicyEnforcer) validateSubagentDepth(run *kubeclawv1alpha1.AgentRun, policy *kubeclawv1alpha1.ClawPolicy) error {
	if policy.Spec.SubagentPolicy == nil || run.Spec.Parent == nil {
		return nil
	}

	if policy.Spec.SubagentPolicy.MaxDepth > 0 && run.Spec.Parent.SpawnDepth >= policy.Spec.SubagentPolicy.MaxDepth {
		return fmt.Errorf("sub-agent depth %d exceeds maximum %d",
			run.Spec.Parent.SpawnDepth, policy.Spec.SubagentPolicy.MaxDepth)
	}

	return nil
}

func (pe *PolicyEnforcer) validateToolPolicy(run *kubeclawv1alpha1.AgentRun, policy *kubeclawv1alpha1.ClawPolicy) error {
	if run.Spec.ToolPolicy == nil || policy.Spec.ToolGating == nil {
		return nil
	}

	// Check that allowed tools in the run spec don't conflict with policy denied tools
	for _, rule := range policy.Spec.ToolGating.Rules {
		if rule.Action == "deny" {
			for _, allowed := range run.Spec.ToolPolicy.Allow {
				if allowed == rule.Tool {
					return fmt.Errorf("tool %q is denied by policy", rule.Tool)
				}
			}
		}
	}

	return nil
}

func (pe *PolicyEnforcer) validateFeatureGates(run *kubeclawv1alpha1.AgentRun, policy *kubeclawv1alpha1.ClawPolicy) error {
	if policy.Spec.FeatureGates == nil {
		return nil
	}

	// Check sandbox feature gate
	if run.Spec.Sandbox != nil && run.Spec.Sandbox.Enabled {
		if enabled, exists := policy.Spec.FeatureGates["code-execution"]; exists && !enabled {
			return fmt.Errorf("feature gate 'code-execution' is disabled by policy")
		}
	}

	// Check sub-agents feature gate
	if run.Spec.Parent != nil {
		if enabled, exists := policy.Spec.FeatureGates["sub-agents"]; exists && !enabled {
			return fmt.Errorf("feature gate 'sub-agents' is disabled by policy")
		}
	}

	return nil
}

// InjectDecoder injects the admission decoder.
func (pe *PolicyEnforcer) InjectDecoder(d admission.Decoder) error {
	pe.decoder = d
	return nil
}

// MutatingPolicyEnforcer is a mutating webhook that injects defaults based on ClawPolicy.
type MutatingPolicyEnforcer struct {
	Client  client.Client
	Log     logr.Logger
	decoder admission.Decoder
}

// Handle mutates AgentRun resources to enforce policy defaults.
func (mpe *MutatingPolicyEnforcer) Handle(ctx context.Context, req admission.Request) admission.Response {
	run := &kubeclawv1alpha1.AgentRun{}
	if err := mpe.decoder.Decode(req, run); err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	// Look up the owning ClawInstance
	var instance kubeclawv1alpha1.ClawInstance
	if err := mpe.Client.Get(ctx, types.NamespacedName{
		Name:      run.Spec.InstanceRef,
		Namespace: run.Namespace,
	}, &instance); err != nil {
		return admission.Allowed("instance not found, skipping mutation")
	}

	if instance.Spec.PolicyRef == "" {
		return admission.Allowed("no policy")
	}

	var policy kubeclawv1alpha1.ClawPolicy
	if err := mpe.Client.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.PolicyRef,
		Namespace: run.Namespace,
	}, &policy); err != nil {
		return admission.Allowed("policy not found, skipping mutation")
	}

	modified := false

	// Inject sandbox defaults
	if policy.Spec.SandboxPolicy != nil && policy.Spec.SandboxPolicy.Required {
		if run.Spec.Sandbox == nil {
			run.Spec.Sandbox = &kubeclawv1alpha1.AgentRunSandboxSpec{
				Enabled: true,
			}
			modified = true
		}
		if policy.Spec.SandboxPolicy.DefaultImage != "" && run.Spec.Sandbox.Image == "" {
			run.Spec.Sandbox.Image = policy.Spec.SandboxPolicy.DefaultImage
			modified = true
		}
	}

	// Inject tool policy defaults from ClawPolicy
	if policy.Spec.ToolGating != nil && run.Spec.ToolPolicy == nil {
		tp := &kubeclawv1alpha1.ToolPolicySpec{}
		for _, rule := range policy.Spec.ToolGating.Rules {
			switch rule.Action {
			case "allow":
				tp.Allow = append(tp.Allow, rule.Tool)
			case "deny":
				tp.Deny = append(tp.Deny, rule.Tool)
			}
		}
		run.Spec.ToolPolicy = tp
		modified = true
	}

	// Inject network isolation labels (used by NetworkPolicy)
	if run.Labels == nil {
		run.Labels = make(map[string]string)
	}
	if _, exists := run.Labels["kubeclaw.io/role"]; !exists {
		run.Labels["kubeclaw.io/role"] = "agent"
		modified = true
	}
	if run.Spec.Sandbox != nil && run.Spec.Sandbox.Enabled {
		run.Labels["kubeclaw.io/sandbox"] = "true"
		modified = true
	}

	// Disable service account token automount via annotation
	if run.Annotations == nil {
		run.Annotations = make(map[string]string)
	}
	if _, exists := run.Annotations["kubeclaw.io/disable-sa-token"]; !exists {
		run.Annotations["kubeclaw.io/disable-sa-token"] = "true"
		modified = true
	}

	if !modified {
		return admission.Allowed("no mutations needed")
	}

	// Create the JSON patch
	marshaledRun, err := json.Marshal(run)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledRun)
}

// InjectDecoder injects the admission decoder.
func (mpe *MutatingPolicyEnforcer) InjectDecoder(d admission.Decoder) error {
	mpe.decoder = d
	return nil
}

// BuildAgentPodSecurityContext returns a restricted SecurityContext for agent pods.
func BuildAgentPodSecurityContext() *corev1.SecurityContext {
	falseBool := false
	return &corev1.SecurityContext{
		AllowPrivilegeEscalation: &falseBool,
		ReadOnlyRootFilesystem:   &falseBool,
		RunAsNonRoot:             boolPtr(true),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}
