package controllers

import (
	"context"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OrganizationReconciler reconciles a Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Keycloak KeycloakPutter
}

//go:generate go run github.com/golang/mock/mockgen -source=$GOFILE -destination=./mock/keycloak-putter.go
type KeycloakPutter interface {
	PutGroup(ctx context.Context, group keycloak.Group) (keycloak.Group, error)
}

//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations/finalizers,verbs=update

func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(4).WithValues("request", req).Info("Reconciling")

	log.V(4).Info("Getting Organization and Members..")
	org, orgMemb, err := r.GetOrganizationAndMembers(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Handle deletion

	group := buildKeycloakGroup(org, orgMemb)

	log.V(4).Info("Reconciling Keycloak group..")
	group, err = r.Keycloak.PutGroup(ctx, group)
	if err != nil {
		return ctrl.Result{}, err
	}

	// TODO: Add finalizer

	return ctrl.Result{}, nil
}

func (r *OrganizationReconciler) GetOrganizationAndMembers(ctx context.Context, orgKey types.NamespacedName) (*orgv1.Organization, *controlv1.OrganizationMembers, error) {
	org := &orgv1.Organization{}
	if err := r.Get(ctx, orgKey, org); err != nil {
		return org, nil, err
	}

	memb := &controlv1.OrganizationMembers{}
	membKey := types.NamespacedName{
		Namespace: org.Name,
		Name:      "members",
	}
	err := r.Get(ctx, membKey, memb)
	return org, memb, err
}

func buildKeycloakGroup(org *orgv1.Organization, memb *controlv1.OrganizationMembers) keycloak.Group {
	groupMem := []string{}

	for _, u := range memb.Spec.UserRefs {
		groupMem = append(groupMem, u.Name)
	}

	return keycloak.Group{
		Name:    org.Name,
		Members: groupMem,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *OrganizationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&orgv1.Organization{}).
		Owns(&controlv1.OrganizationMembers{}).
		Complete(r)
}
