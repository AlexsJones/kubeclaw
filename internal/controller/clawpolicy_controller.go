package controller

import (
	"context"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	k8sclawv1alpha1 "github.com/k8sclaw/k8sclaw/api/v1alpha1"
)

// ClawPolicyReconciler reconciles a ClawPolicy object.
type ClawPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

// +kubebuilder:rbac:groups=k8sclaw.io,resources=clawpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=k8sclaw.io,resources=clawpolicies/status,verbs=get;update;patch

// Reconcile handles ClawPolicy reconciliation.
func (r *ClawPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("clawpolicy", req.NamespacedName)

	var policy k8sclawv1alpha1.ClawPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Count ClawInstances that reference this policy
	var instances k8sclawv1alpha1.ClawInstanceList
	if err := r.List(ctx, &instances, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	bound := 0
	for _, inst := range instances.Items {
		if inst.Spec.PolicyRef == policy.Name {
			bound++
		}
	}

	// Update status
	policy.Status.BoundInstances = bound
	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Error(err, "failed to update policy status")
		return ctrl.Result{}, err
	}

	log.Info("Reconciled ClawPolicy", "boundInstances", bound)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClawPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&k8sclawv1alpha1.ClawPolicy{}).
		Complete(r)
}
