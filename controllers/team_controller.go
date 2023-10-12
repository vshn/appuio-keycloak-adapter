package controllers

import (
	"context"
	"errors"

	controlv1 "github.com/appuio/control-api/apis/v1"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// TeamReconciler reconciles a Team object
type TeamReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme

	Keycloak KeycloakClient
}

//+kubebuilder:rbac:groups=appuio.io,resources=teams,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=appuio.io,resources=teams/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=appuio.io,resources=teams/finalizers,verbs=update

// Reconcile reacts on changes of teams and mirrors these changes to groups in Keycloak
func (r *TeamReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.V(4).WithValues("request", req).Info("Reconciling")

	log.V(4).Info("Getting the Team..")
	team := &controlv1.Team{}
	if err := r.Get(ctx, req.NamespacedName, team); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !team.ObjectMeta.DeletionTimestamp.IsZero() {
		log.V(4).Info("Deleting Keycloak group..")
		err := r.Keycloak.DeleteGroup(ctx, team.Namespace, team.Name)
		if err != nil {
			r.Recorder.Event(team, "Warning", "DeletionFailed", "Failed to delete Keycloak Group")
			return ctrl.Result{}, err
		}

		err = r.removeFinalizer(ctx, team)
		return ctrl.Result{}, err
	}
	err := r.addFinalizer(ctx, team)
	if err != nil {
		return ctrl.Result{}, err
	}

	log.V(4).Info("Reconciling Keycloak group..")
	group, err := r.Keycloak.PutGroup(ctx, buildTeamKeycloakGroup(team))
	var membErrs *keycloak.MembershipSyncErrors
	if errors.As(err, &membErrs) {
		for _, membErr := range *membErrs {
			r.Recorder.Eventf(team, "Warning", string(membErr.Event), "Failed to update membership of user %s", membErr.Username)
			log.Error(membErr, "Failed to update membership", "user", membErr.Username)
		}
	} else if err != nil {
		r.Recorder.Event(team, "Warning", "UpdateFailed", "Failed to update Keycloak Group")
		return ctrl.Result{}, err
	}

	log.V(4).Info("Updating status..")
	err = r.updateTeamStatus(ctx, team, group)
	return ctrl.Result{}, err
}

func (r *TeamReconciler) addFinalizer(ctx context.Context, team client.Object) error {
	if !controllerutil.ContainsFinalizer(team, orgFinalizer) {
		controllerutil.AddFinalizer(team, orgFinalizer)
		if err := r.Update(ctx, team); err != nil {
			return err
		}
	}
	return nil
}

func (r *TeamReconciler) removeFinalizer(ctx context.Context, team client.Object) error {
	if controllerutil.ContainsFinalizer(team, orgFinalizer) {
		controllerutil.RemoveFinalizer(team, orgFinalizer)
		if err := r.Update(ctx, team); err != nil {
			return err
		}
	}
	return nil
}

func (r *TeamReconciler) updateTeamStatus(ctx context.Context, team *controlv1.Team, group keycloak.Group) error {
	userRefs := make([]controlv1.UserRef, 0, len(group.Members))
	for _, u := range group.Members {
		userRefs = append(userRefs, controlv1.UserRef{
			Name: u.Username,
		})
	}
	team.Status.ResolvedUserRefs = userRefs
	return r.Status().Update(ctx, team)
}

func buildTeamKeycloakGroup(team *controlv1.Team) keycloak.Group {
	groupMem := make([]string, 0, len(team.Spec.UserRefs))

	for _, u := range team.Spec.UserRefs {
		groupMem = append(groupMem, u.Name)
	}

	return keycloak.NewGroup(team.Spec.DisplayName, team.Namespace, team.Name).WithMemberNames(groupMem...)
}

// SetupWithManager sets up the controller with the Manager.
func (r *TeamReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&controlv1.Team{}).
		Complete(r)
}
