package controllers

import (
	"context"

	controlv1 "github.com/appuio/control-api/apis/v1"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// UserReconciler reconciles a User object
type UserReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme

	Keycloak KeycloakClient
}

//+kubebuilder:rbac:groups=appuio.io,resources=users,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=appuio.io,resources=users/status,verbs=get;update;patch

// Reconcile reacts on changes of users and mirrors these changes to Keycloak
func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(4).WithValues("request", req).Info("Reconciling")

	log.V(4).Info("Getting the User..")
	user := controlv1.User{}
	if err := r.Get(ctx, req.NamespacedName, &user); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !user.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	log.V(4).Info("Reconciling Keycloak group..")
	kcUser, err := r.Keycloak.PutUser(ctx, buildKeycloakUser(user))
	if err != nil {
		r.Recorder.Event(&user, "Warning", "UpdateFailed", "Failed to update Keycloak User")
		return ctrl.Result{}, err
	}

	log.V(4).Info("Updating status..")
	err = r.updateUserStatus(ctx, user, kcUser)
	return ctrl.Result{}, err
}

func (r *UserReconciler) updateUserStatus(ctx context.Context, user controlv1.User, kcUser keycloak.User) error {
	user.Status.ID = kcUser.ID
	user.Status.Username = kcUser.Username
	user.Status.Email = kcUser.Email
	user.Status.DisplayName = kcUser.DisplayName()
	user.Status.DefaultOrganizationRef = kcUser.DefaultOrganizationRef
	return r.Status().Update(ctx, &user)
}

func buildKeycloakUser(u controlv1.User) keycloak.User {
	return keycloak.User{
		Username:               u.Name,
		DefaultOrganizationRef: u.Spec.Preferences.DefaultOrganizationRef,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlv1.User{}).
		Complete(r)
}
