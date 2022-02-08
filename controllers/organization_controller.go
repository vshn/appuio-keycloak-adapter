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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OrganizationReconciler reconciles a Organization object
type OrganizationReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Keycloak KeycloakClient
}

//go:generate go run github.com/golang/mock/mockgen -source=$GOFILE -destination=./ZZ_mock_keycloak_test.go -package controllers_test
type KeycloakClient interface {
	PutGroup(ctx context.Context, group keycloak.Group) (keycloak.Group, error)
	DeleteGroup(ctx context.Context, groupName string) error
}

var orgFinalizer = "keycloak-adapter.vshn.net/finalizer"

//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=organization.appuio.io,resources=organizations/finalizers,verbs=update

func (r *OrganizationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(4).WithValues("request", req).Info("Reconciling")

	log.V(4).Info("Getting Organization and Members..")
	org, orgMemb, err := r.getOrganizationAndMembers(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !org.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("Deleting Keycloak group..")
		err = r.Keycloak.DeleteGroup(ctx, org.Name)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.removeFinalizer(ctx, org, orgMemb)
		return ctrl.Result{}, err
	}
	err = r.addFinalizer(ctx, org, orgMemb)
	if err != nil {
		return ctrl.Result{}, err
	}

	group := buildKeycloakGroup(org, orgMemb)

	log.V(4).Info("Reconciling Keycloak group..")
	group, err = r.Keycloak.PutGroup(ctx, group)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.V(4).Info("Updating status..")
	err = r.updateOrganizationStatus(ctx, org, orgMemb, group)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *OrganizationReconciler) getOrganizationAndMembers(ctx context.Context, orgKey types.NamespacedName) (*orgv1.Organization, *controlv1.OrganizationMembers, error) {
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

func (r *OrganizationReconciler) addFinalizer(ctx context.Context, org *orgv1.Organization, memb *controlv1.OrganizationMembers) error {
	if !controllerutil.ContainsFinalizer(memb, orgFinalizer) {
		controllerutil.AddFinalizer(memb, orgFinalizer)
		if err := r.Update(ctx, memb); err != nil {
			return err
		}
	}
	if !controllerutil.ContainsFinalizer(org, orgFinalizer) {
		controllerutil.AddFinalizer(org, orgFinalizer)
		if err := r.Update(ctx, org); err != nil {
			return err
		}
	}
	return nil
}
func (r *OrganizationReconciler) removeFinalizer(ctx context.Context, org *orgv1.Organization, memb *controlv1.OrganizationMembers) error {
	if controllerutil.ContainsFinalizer(org, orgFinalizer) {
		controllerutil.RemoveFinalizer(org, orgFinalizer)
		if err := r.Update(ctx, org); err != nil {
			return err
		}
	}
	if controllerutil.ContainsFinalizer(memb, orgFinalizer) {
		controllerutil.RemoveFinalizer(memb, orgFinalizer)
		if err := r.Update(ctx, memb); err != nil {
			return err
		}
	}
	return nil
}

func (r *OrganizationReconciler) updateOrganizationStatus(ctx context.Context, org *orgv1.Organization, memb *controlv1.OrganizationMembers, group keycloak.Group) error {
	userRefs := []controlv1.UserRef{}
	for _, u := range group.Members {
		userRefs = append(userRefs, controlv1.UserRef{
			Name: u,
		})
	}
	memb.Status.ResolvedUserRefs = userRefs
	return r.Status().Update(ctx, memb)
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
