/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	platformv1alpha1 "github.com/aasthakumarii/mcago/api/v1alpha1"
	"github.com/aasthakumarii/mcago/internal/awsiam"
	"github.com/aasthakumarii/mcago/internal/models"
	"github.com/aasthakumarii/mcago/internal/service"
)

const teamFinalizer = "platform.mcago.dev/finalizer"

// TeamReconciler reconciles a Team object
type TeamReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Service    *service.TeamService
	IAMManager *awsiam.IAMManager
}

// +kubebuilder:rbac:groups=platform.mcago.dev,resources=teams,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=platform.mcago.dev,resources=teams/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=platform.mcago.dev,resources=teams/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *TeamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var team platformv1alpha1.Team
	if err := r.Get(ctx, req.NamespacedName, &team); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion / finalizer cleanup.
	if !team.ObjectMeta.DeletionTimestamp.IsZero() {
		if containsString(team.Finalizers, teamFinalizer) {
			if err := r.cleanup(ctx, &team); err != nil {
				return ctrl.Result{}, err
			}
			team.Finalizers = removeString(team.Finalizers, teamFinalizer)
			if err := r.Update(ctx, &team); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !containsString(team.Finalizers, teamFinalizer) {
		team.Finalizers = append(team.Finalizers, teamFinalizer)
		if err := r.Update(ctx, &team); err != nil {
			return ctrl.Result{}, err
		}
	}

	domainTeam := models.Team{
		Name:        team.Name,
		Namespace:   team.Spec.Namespace,
		Description: team.Spec.Description,
		Owner:       team.Spec.Owner,
		Labels:      team.Spec.Labels,
	}

	plan, err := r.Service.PlanReconcile(ctx, domainTeam)
	if err != nil {
		r.setCondition(&team, platformv1alpha1.ConditionReady, metav1.ConditionFalse, "ValidationFailed", err.Error())
		_ = r.Status().Update(ctx, &team)
		return ctrl.Result{}, err
	}

	if err := r.ensureNamespace(ctx, &team, plan); err != nil {
		logger.Error(err, "failed to ensure namespace")
		return ctrl.Result{}, err
	}
	if err := r.ensureServiceAccount(ctx, &team, plan); err != nil {
		logger.Error(err, "failed to ensure service account")
		return ctrl.Result{}, err
	}
	if err := r.ensureRBAC(ctx, &team, plan); err != nil {
		logger.Error(err, "failed to ensure rbac")
		return ctrl.Result{}, err
	}
	if err := r.ensureIAMRole(ctx, &team, plan); err != nil {
		logger.Error(err, "failed to ensure IAM role")
		return ctrl.Result{}, err
	}

	r.setCondition(&team, platformv1alpha1.ConditionReady, metav1.ConditionTrue, "Reconciled", "team is ready")
	team.Status.ObservedGeneration = team.Generation
	if err := r.Status().Update(ctx, &team); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *TeamReconciler) ensureNamespace(ctx context.Context, team *platformv1alpha1.Team, plan service.TeamResources) error {
	ns := &corev1.Namespace{}
	err := r.Get(ctx, types.NamespacedName{Name: plan.NamespaceName}, ns)
	if apierrors.IsNotFound(err) {
		ns = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: plan.NamespaceName, Labels: plan.Labels},
		}
		if err := r.Create(ctx, ns); err != nil {
			r.setCondition(team, platformv1alpha1.ConditionNamespaceCreated, metav1.ConditionFalse, "CreateFailed", err.Error())
			return err
		}
		r.setCondition(team, platformv1alpha1.ConditionNamespaceCreated, metav1.ConditionTrue, "Created", "namespace created")
		return nil
	}
	if err != nil {
		return err
	}
	r.setCondition(team, platformv1alpha1.ConditionNamespaceCreated, metav1.ConditionTrue, "AlreadyExists", "namespace already exists")
	return nil
}

func (r *TeamReconciler) ensureServiceAccount(ctx context.Context, team *platformv1alpha1.Team, plan service.TeamResources) error {
	sa := &corev1.ServiceAccount{}
	key := types.NamespacedName{Name: plan.ServiceAccountName, Namespace: plan.NamespaceName}
	err := r.Get(ctx, key, sa)
	if apierrors.IsNotFound(err) {
		sa = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: plan.ServiceAccountName, Namespace: plan.NamespaceName, Labels: plan.Labels},
		}
		if err := r.Create(ctx, sa); err != nil {
			r.setCondition(team, platformv1alpha1.ConditionServiceAccountCreated, metav1.ConditionFalse, "CreateFailed", err.Error())
			return err
		}
		r.setCondition(team, platformv1alpha1.ConditionServiceAccountCreated, metav1.ConditionTrue, "Created", "service account created")
		return nil
	}
	if err != nil {
		return err
	}
	r.setCondition(team, platformv1alpha1.ConditionServiceAccountCreated, metav1.ConditionTrue, "AlreadyExists", "service account already exists")
	return nil
}

func (r *TeamReconciler) ensureRBAC(ctx context.Context, team *platformv1alpha1.Team, plan service.TeamResources) error {
	role := &rbacv1.Role{}
	roleKey := types.NamespacedName{Name: plan.RoleName, Namespace: plan.NamespaceName}
	err := r.Get(ctx, roleKey, role)
	if apierrors.IsNotFound(err) {
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{Name: plan.RoleName, Namespace: plan.NamespaceName, Labels: plan.Labels},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "services", "configmaps", "secrets"},
					Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
				},
			},
		}
		if err := r.Create(ctx, role); err != nil {
			r.setCondition(team, platformv1alpha1.ConditionRBACCreated, metav1.ConditionFalse, "CreateFailed", err.Error())
			return err
		}
	} else if err != nil {
		return err
	}

	rb := &rbacv1.RoleBinding{}
	rbKey := types.NamespacedName{Name: plan.RoleBindingName, Namespace: plan.NamespaceName}
	err = r.Get(ctx, rbKey, rb)
	if apierrors.IsNotFound(err) {
		rb = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: plan.RoleBindingName, Namespace: plan.NamespaceName, Labels: plan.Labels},
			Subjects: []rbacv1.Subject{
				{Kind: "ServiceAccount", Name: plan.ServiceAccountName, Namespace: plan.NamespaceName},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     plan.RoleName,
			},
		}
		if err := r.Create(ctx, rb); err != nil {
			r.setCondition(team, platformv1alpha1.ConditionRBACCreated, metav1.ConditionFalse, "CreateFailed", err.Error())
			return err
		}
		r.setCondition(team, platformv1alpha1.ConditionRBACCreated, metav1.ConditionTrue, "Created", "rbac created")
		return nil
	}
	if err != nil {
		return err
	}
	r.setCondition(team, platformv1alpha1.ConditionRBACCreated, metav1.ConditionTrue, "AlreadyExists", "rbac already exists")
	return nil
}

func (r *TeamReconciler) ensureIAMRole(ctx context.Context, team *platformv1alpha1.Team, plan service.TeamResources) error {
	if err := r.IAMManager.EnsureTeamRole(ctx, plan.IAMRoleName); err != nil {
		r.setCondition(team, platformv1alpha1.ConditionIAMRoleCreated, metav1.ConditionFalse, "CreateFailed", err.Error())
		return err
	}
	r.setCondition(team, platformv1alpha1.ConditionIAMRoleCreated, metav1.ConditionTrue, "Created", "iam role ready")
	return nil
}

func (r *TeamReconciler) cleanup(ctx context.Context, team *platformv1alpha1.Team) error {
	domainTeam := models.Team{
		Name:        team.Name,
		Namespace:   team.Spec.Namespace,
		Description: team.Spec.Description,
		Owner:       team.Spec.Owner,
		Labels:      team.Spec.Labels,
	}

	// Reconstruct the plan to get the exact generated IAM Role Name
	plan, err := r.Service.PlanReconcile(ctx, domainTeam)
	if err == nil {
		if err := r.IAMManager.DeleteTeamRole(ctx, plan.IAMRoleName); err != nil {
			return err
		}
	}

	ns := &corev1.Namespace{}
	err = r.Get(ctx, types.NamespacedName{Name: team.Spec.Namespace}, ns)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := r.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *TeamReconciler) setCondition(team *platformv1alpha1.Team, condType string, status metav1.ConditionStatus, reason, message string) {
	newCond := metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: team.Generation,
	}
	existing := findCondition(team.Status.Conditions, condType)
	if existing == nil {
		newCond.LastTransitionTime = metav1.Now()
		team.Status.Conditions = append(team.Status.Conditions, newCond)
		return
	}
	if existing.Status != newCond.Status {
		existing.LastTransitionTime = metav1.Now()
	}
	existing.Status = newCond.Status
	existing.Reason = newCond.Reason
	existing.Message = newCond.Message
	existing.ObservedGeneration = newCond.ObservedGeneration
}

func findCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *TeamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Service == nil {
		r.Service = service.NewTeamService()
	}
	if r.IAMManager == nil {
		iamMgr, err := awsiam.NewIAMManager(context.Background())
		if err != nil {
			return fmt.Errorf("failed to initialize AWS IAM manager: %w", err)
		}
		r.IAMManager = iamMgr
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&platformv1alpha1.Team{}).
		Owns(&corev1.Namespace{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Named("team").
		Complete(r)
}
