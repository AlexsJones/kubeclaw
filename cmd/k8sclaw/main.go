// Package main provides the k8sclaw CLI tool for managing K8sClaw resources.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sclawv1alpha1 "github.com/k8sclaw/k8sclaw/api/v1alpha1"
)

var (
	kubeconfig string
	namespace  string
	k8sClient  client.Client
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "k8sclaw",
		Short: "K8sClaw - Kubernetes-native AI agent management",
		Long: `K8sClaw CLI for managing ClawInstances, AgentRuns, ClawPolicies,
SkillPacks, and feature gates in your Kubernetes cluster.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initClient()
		},
	}

	rootCmd.PersistentFlags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")

	rootCmd.AddCommand(
		newInstancesCmd(),
		newRunsCmd(),
		newPoliciesCmd(),
		newSkillsCmd(),
		newFeaturesCmd(),
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func initClient() error {
	scheme := runtime.NewScheme()
	if err := k8sclawv1alpha1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to register scheme: %w", err)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if kubeconfig != "" {
		loadingRules.ExplicitPath = kubeconfig
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, &clientcmd.ConfigOverrides{},
	).ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	c, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	k8sClient = c
	return nil
}

func newInstancesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "instances",
		Aliases: []string{"instance", "inst"},
		Short:   "Manage ClawInstances",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List ClawInstances",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var list k8sclawv1alpha1.ClawInstanceList
				if err := k8sClient.List(ctx, &list, client.InNamespace(namespace)); err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tPHASE\tCHANNELS\tAGENT PODS\tAGE")
				for _, inst := range list.Items {
					age := time.Since(inst.CreationTimestamp.Time).Round(time.Second)
					channels := make([]string, 0)
					for _, ch := range inst.Status.Channels {
						channels = append(channels, ch.Type)
					}
					fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
						inst.Name, inst.Status.Phase,
						strings.Join(channels, ","),
						inst.Status.ActiveAgentPods, age)
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "get [name]",
			Short: "Get a ClawInstance",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var inst k8sclawv1alpha1.ClawInstance
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: namespace}, &inst); err != nil {
					return err
				}
				data, _ := json.MarshalIndent(inst, "", "  ")
				fmt.Println(string(data))
				return nil
			},
		},
		&cobra.Command{
			Use:   "delete [name]",
			Short: "Delete a ClawInstance",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				inst := &k8sclawv1alpha1.ClawInstance{
					ObjectMeta: metav1.ObjectMeta{Name: args[0], Namespace: namespace},
				}
				if err := k8sClient.Delete(ctx, inst); err != nil {
					return err
				}
				fmt.Printf("clawinstance/%s deleted\n", args[0])
				return nil
			},
		},
	)
	return cmd
}

func newRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "runs",
		Aliases: []string{"run"},
		Short:   "Manage AgentRuns",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List AgentRuns",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var list k8sclawv1alpha1.AgentRunList
				if err := k8sClient.List(ctx, &list, client.InNamespace(namespace)); err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tINSTANCE\tPHASE\tPOD\tAGE")
				for _, run := range list.Items {
					age := time.Since(run.CreationTimestamp.Time).Round(time.Second)
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
						run.Name, run.Spec.InstanceRef,
						run.Status.Phase, run.Status.PodName, age)
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "get [name]",
			Short: "Get an AgentRun",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var run k8sclawv1alpha1.AgentRun
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: namespace}, &run); err != nil {
					return err
				}
				data, _ := json.MarshalIndent(run, "", "  ")
				fmt.Println(string(data))
				return nil
			},
		},
		&cobra.Command{
			Use:   "logs [name]",
			Short: "Stream logs from an AgentRun pod",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var run k8sclawv1alpha1.AgentRun
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: namespace}, &run); err != nil {
					return err
				}
				if run.Status.PodName == "" {
					return fmt.Errorf("agentrun %s has no pod yet (phase: %s)", args[0], run.Status.Phase)
				}
				fmt.Printf("Use: kubectl logs %s -c agent -f\n", run.Status.PodName)
				return nil
			},
		},
	)
	return cmd
}

func newPoliciesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "policies",
		Aliases: []string{"policy", "pol"},
		Short:   "Manage ClawPolicies",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List ClawPolicies",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var list k8sclawv1alpha1.ClawPolicyList
				if err := k8sClient.List(ctx, &list, client.InNamespace(namespace)); err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tBOUND INSTANCES\tAGE")
				for _, pol := range list.Items {
					age := time.Since(pol.CreationTimestamp.Time).Round(time.Second)
					fmt.Fprintf(w, "%s\t%d\t%s\n", pol.Name, pol.Status.BoundInstances, age)
				}
				return w.Flush()
			},
		},
		&cobra.Command{
			Use:   "get [name]",
			Short: "Get a ClawPolicy",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var pol k8sclawv1alpha1.ClawPolicy
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: args[0], Namespace: namespace}, &pol); err != nil {
					return err
				}
				data, _ := json.MarshalIndent(pol, "", "  ")
				fmt.Println(string(data))
				return nil
			},
		},
	)
	return cmd
}

func newSkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "skills",
		Aliases: []string{"skill", "sk"},
		Short:   "Manage SkillPacks",
	}

	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List SkillPacks",
			RunE: func(cmd *cobra.Command, args []string) error {
				ctx := context.Background()
				var list k8sclawv1alpha1.SkillPackList
				if err := k8sClient.List(ctx, &list, client.InNamespace(namespace)); err != nil {
					return err
				}
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintln(w, "NAME\tSKILLS\tCONFIGMAP\tAGE")
				for _, sk := range list.Items {
					age := time.Since(sk.CreationTimestamp.Time).Round(time.Second)
					fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
						sk.Name, len(sk.Spec.Skills), sk.Status.ConfigMapName, age)
				}
				return w.Flush()
			},
		},
	)
	return cmd
}

func newFeaturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "features",
		Aliases: []string{"feature", "feat"},
		Short:   "Manage feature gates",
	}

	enableCmd := &cobra.Command{
		Use:   "enable [feature]",
		Short: "Enable a feature gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleFeature(args[0], true, cmd)
		},
	}
	enableCmd.Flags().String("policy", "", "Target ClawPolicy")

	disableCmd := &cobra.Command{
		Use:   "disable [feature]",
		Short: "Disable a feature gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return toggleFeature(args[0], false, cmd)
		},
	}
	disableCmd.Flags().String("policy", "", "Target ClawPolicy")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List feature gates on a policy",
		RunE: func(cmd *cobra.Command, args []string) error {
			policyName, _ := cmd.Flags().GetString("policy")
			if policyName == "" {
				return fmt.Errorf("--policy is required")
			}
			ctx := context.Background()
			var pol k8sclawv1alpha1.ClawPolicy
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: namespace}, &pol); err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "FEATURE\tENABLED")
			if pol.Spec.FeatureGates != nil {
				for feature, enabled := range pol.Spec.FeatureGates {
					fmt.Fprintf(w, "%s\t%v\n", feature, enabled)
				}
			}
			return w.Flush()
		},
	}
	listCmd.Flags().String("policy", "", "Target ClawPolicy")

	cmd.AddCommand(enableCmd, disableCmd, listCmd)
	return cmd
}

func toggleFeature(feature string, enabled bool, cmd *cobra.Command) error {
	policyName, _ := cmd.Flags().GetString("policy")
	if policyName == "" {
		return fmt.Errorf("--policy is required")
	}

	ctx := context.Background()
	var pol k8sclawv1alpha1.ClawPolicy
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: namespace}, &pol); err != nil {
		return err
	}

	if pol.Spec.FeatureGates == nil {
		pol.Spec.FeatureGates = make(map[string]bool)
	}
	pol.Spec.FeatureGates[feature] = enabled

	if err := k8sClient.Update(ctx, &pol); err != nil {
		return err
	}

	action := "enabled"
	if !enabled {
		action = "disabled"
	}
	fmt.Printf("Feature %q %s on policy %s\n", feature, action, policyName)
	return nil
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("k8sclaw v0.1.0-dev")
		},
	}
}
