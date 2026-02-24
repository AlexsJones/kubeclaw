package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

const defaultSkillsDir = "/skills"

// loadSkills reads all skill files from the skills directory and returns
// their concatenated content suitable for prepending to the system prompt.
func loadSkills(skillsDir string) string {
	if skillsDir == "" {
		skillsDir = defaultSkillsDir
	}

	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		log.Printf("No skills directory at %s: %v", skillsDir, err)
		return ""
	}

	var sb strings.Builder
	count := 0
	for _, entry := range entries {
		// Skip directories and hidden files (Kubernetes projected volumes
		// create ..data, ..timestamp, etc.).
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(skillsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("Failed to read skill file %s: %v", path, err)
			continue
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n---\n\n")
		}
		sb.WriteString(content)
		count++
	}

	if count > 0 {
		log.Printf("Loaded %d skill file(s) from %s", count, skillsDir)
	}
	return sb.String()
}

// buildSystemPrompt assembles the full system prompt from the base prompt,
// loaded skills, and tool availability.
func buildSystemPrompt(base string, skills string, toolsEnabled bool) string {
	var sb strings.Builder

	sb.WriteString(base)

	if skills != "" {
		sb.WriteString("\n\n## Your Skills\n\n")
		sb.WriteString("The following skill instructions have been loaded. Follow them when they are relevant to the task:\n\n")
		sb.WriteString(skills)
	}

	if toolsEnabled {
		sb.WriteString("\n\n## Tool Usage\n\n")
		sb.WriteString("You have access to tools that let you execute commands and inspect files. ")
		sb.WriteString("When the task requires interacting with Kubernetes or running shell commands, ")
		sb.WriteString("use the `execute_command` tool to run them. The commands run inside a sidecar container ")
		sb.WriteString("that has kubectl and other CLI tools available.\n\n")
		sb.WriteString("**Important: You are running inside a Kubernetes pod with full cluster admin access. ")
		sb.WriteString("kubectl is pre-configured via a mounted ServiceAccount token and works out of the box. ")
		sb.WriteString("You have RBAC permissions to read all resources cluster-wide and manage workloads in any namespace. ")
		sb.WriteString("Do NOT check kubeconfig, contexts, or try to configure cluster access — just run kubectl commands directly. ")
		sb.WriteString("Commands like `kubectl get pods -A` and `kubectl get nodes` work. ")
		sb.WriteString("`kubectl config current-context` will always error in-cluster; this is normal and expected.**\n\n")
		sb.WriteString("Always use tools to gather real information rather than guessing. ")
		sb.WriteString("For example, if asked about pod status, run `kubectl get pods` rather than speculating.\n\n")
		sb.WriteString("After executing commands, summarise the results clearly for the user.")
	}

	return sb.String()
}
