package controllers

import (
	"context"
	"fmt"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"
	"go.uber.org/multierr"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/vshn/appuio-keycloak-adapter/keycloak"
)

const orgImportAnnot = "keycloak-adapter.vshn.net/importing"

// PeriodicSyncer reconciles a Organization object
type PeriodicSyncer struct {
	client.Client
	Recorder record.EventRecorder

	Keycloak KeycloakClient

	// SyncClusterRoles to give to group members when importing
	SyncClusterRoles           []string
	SyncClusterRolesUserPrefix string
}

//+kubebuilder:rbac:groups=appuio.io,resources=organizationmembers,verbs=create
//+kubebuilder:rbac:groups=appuio.io,resources=teams,verbs=create
//+kubebuilder:rbac:groups=appuio.io,resources=users,verbs=create
// Allows managing other users than the one used by the controller itself
// See https://github.com/appuio/control-api/pull/163
//+kubebuilder:rbac:groups=rbac.appuio.io,resources=users,verbs=create
//+kubebuilder:rbac:groups=organization.appuio.io;rbac.appuio.io,resources=organizations,verbs=create
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=subjects;rolebindings,verbs=*

// Sync lists all Keycloak groups in the realm and creates corresponding Organizations if they do not exist
func (r *PeriodicSyncer) Sync(ctx context.Context) error {
	logger := log.FromContext(ctx)

	gs, err := r.Keycloak.ListGroups(ctx)
	if err != nil {
		return fmt.Errorf("cannot list Keycloak groups: %w", err)
	}

	orgMap, err := r.fetchOrganizationMap(ctx)
	if err != nil {
		return fmt.Errorf("cannot list Organizations: %w", err)
	}

	var groupErr error
	for _, g := range gs {
		org, err := r.syncGroup(ctx, g, orgMap)
		if err != nil {
			logger.WithValues("group", g).Error(err, "import of group failed")
			if org != nil {
				r.Recorder.Event(org, "Warning", "ImportFailed", err.Error())
			}
			groupErr = multierr.Append(groupErr, fmt.Errorf("%w\n%s: %s", groupErr, g.BaseName(), err.Error()))
		}
	}
	userErr := r.createMissingUsers(ctx, gs)

	if err := multierr.Append(groupErr, userErr); err != nil {
		return fmt.Errorf("partial sync failure:\n%w", err)
	}

	return nil
}

func (r *PeriodicSyncer) createMissingUsers(ctx context.Context, groups []keycloak.Group) error {
	existing, err := r.fetchAPIUsers(ctx)
	if err != nil {
		return fmt.Errorf("cannot list Users: %w", err)
	}

	var createErr error
	for _, g := range groups {
		for _, m := range g.Members {
			if _, exists := existing[m.Username]; exists {
				continue
			}
			err := r.createUser(ctx, m)
			if err != nil {
				createErr = multierr.Append(createErr, err)
				continue
			}
			existing[m.Username] = struct{}{}
		}
	}

	return createErr
}

func (r *PeriodicSyncer) createUser(ctx context.Context, m keycloak.User) error {
	return r.Create(ctx, &controlv1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: m.Username,
		},
		Spec: controlv1.UserSpec{
			Preferences: controlv1.UserPreferences{
				DefaultOrganizationRef: m.DefaultOrganizationRef,
			},
		},
	})
}

func (r *PeriodicSyncer) syncGroup(ctx context.Context, g keycloak.Group, orgMap map[string]*orgv1.Organization) (runtime.Object, error) {
	logger := log.FromContext(ctx)

	const depth = 0
	switch len(g.PathMembers()) - depth {
	case 1:
		return r.syncOrganization(ctx, g, orgMap[g.BaseName()])
	case 2:
		return r.syncTeam(ctx, g)
	}

	logger.Info("skipped syncing group. invalid hierarchy", "group", g)
	return nil, nil
}

func (r *PeriodicSyncer) syncTeam(ctx context.Context, g keycloak.Group) (*controlv1.Team, error) {
	logger := log.FromContext(ctx)
	var err error

	path := g.PathMembers()
	teamKey := types.NamespacedName{Namespace: path[len(path)-2], Name: path[len(path)-1]}

	team := &controlv1.Team{}
	err = r.Client.Get(ctx, teamKey, team)
	if err != nil && apierrors.IsNotFound(err) {
		logger.V(1).WithValues("group", g).Info("creating team")
		t, err := r.createTeam(ctx, teamKey.Namespace, teamKey.Name, g.Members)
		if err != nil {
			return nil, fmt.Errorf("error creating team %+v: %w", teamKey, err)
		}
		team = t
	} else if err != nil {
		return nil, fmt.Errorf("error getting team %+v: %w", teamKey, err)
	}

	return team, nil
}

func (r *PeriodicSyncer) syncOrganization(ctx context.Context, g keycloak.Group, org *orgv1.Organization) (*orgv1.Organization, error) {
	logger := log.FromContext(ctx)
	var err error

	if org == nil {
		logger.V(1).WithValues("group", g).Info("creating organization")
		org, err = r.startImportOrganizationFromGroup(ctx, g)
		if err != nil {
			return org, err
		}
	}
	if org.Annotations[orgImportAnnot] == "true" {
		logger.V(1).WithValues("group", g).Info("updating organization members")
		err := r.updateOrganizationMembersFromGroup(ctx, g)
		if err != nil {
			return org, err
		}
		err = r.setRolebindingsFromGroup(ctx, g)
		if err != nil {
			return org, err
		}
		err = r.finishImportOrganizationFromGroup(ctx, org)
		if err != nil {
			return org, err
		}
	}
	return org, err
}

func (r *PeriodicSyncer) fetchOrganizationMap(ctx context.Context) (map[string]*orgv1.Organization, error) {
	orgs := orgv1.OrganizationList{}
	err := r.List(ctx, &orgs)
	if err != nil {
		return nil, err
	}

	orgMap := map[string]*orgv1.Organization{}
	for i, o := range orgs.Items {
		orgMap[o.Name] = &orgs.Items[i]
	}
	return orgMap, nil
}

func (r *PeriodicSyncer) fetchAPIUsers(ctx context.Context) (map[string]struct{}, error) {
	users := controlv1.UserList{}
	err := r.List(ctx, &users)
	if err != nil {
		return nil, err
	}

	userMap := map[string]struct{}{}
	for _, u := range users.Items {
		userMap[u.Name] = struct{}{}
	}
	return userMap, nil
}

func (r *PeriodicSyncer) createTeam(ctx context.Context, namespace, name string, members []keycloak.User) (*controlv1.Team, error) {
	team := &controlv1.Team{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: controlv1.TeamSpec{
			DisplayName: name,
		},
	}

	team.Spec.UserRefs = make([]controlv1.UserRef, len(members))
	for i, m := range members {
		team.Spec.UserRefs[i] = controlv1.UserRef{Name: m.Username}
	}
	err := r.Create(ctx, team)
	return team, err
}

func (r *PeriodicSyncer) startImportOrganizationFromGroup(ctx context.Context, group keycloak.Group) (*orgv1.Organization, error) {
	org := &orgv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: group.BaseName(),
			Annotations: map[string]string{
				orgImportAnnot: "true",
			},
		},
		Spec: orgv1.OrganizationSpec{
			DisplayName: group.BaseName(),
		},
	}
	err := r.Create(ctx, org)
	return org, err
}

func (r *PeriodicSyncer) finishImportOrganizationFromGroup(ctx context.Context, org *orgv1.Organization) error {
	delete(org.Annotations, orgImportAnnot)
	return r.Update(ctx, org)

}

func (r *PeriodicSyncer) updateOrganizationMembersFromGroup(ctx context.Context, group keycloak.Group) error {
	orgMemb := controlv1.OrganizationMembers{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: group.BaseName(),
		Name:      "members",
	}, &orgMemb)
	if err != nil {
		return err
	}
	orgMemb.Spec.UserRefs = make([]controlv1.UserRef, len(group.Members))
	for i, m := range group.Members {
		orgMemb.Spec.UserRefs[i] = controlv1.UserRef{Name: m.Username}
	}
	return r.Update(ctx, &orgMemb)
}

func (r *PeriodicSyncer) setRolebindingsFromGroup(ctx context.Context, group keycloak.Group) error {
	subjects := []rbacv1.Subject{}
	for _, m := range group.Members {
		subjects = append(subjects, rbacv1.Subject{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     r.SyncClusterRolesUserPrefix + m.Username,
		})
	}

	for _, rbName := range r.SyncClusterRoles {

		rb := rbacv1.RoleBinding{}
		err := r.Get(ctx, types.NamespacedName{Namespace: group.BaseName(), Name: rbName}, &rb)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if apierrors.IsNotFound(err) {
			rb := rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: group.BaseName(),
					Name:      rbName,
				},
				Subjects: subjects,
				RoleRef: rbacv1.RoleRef{
					Kind:     "ClusterRole",
					APIGroup: rbacv1.GroupName,
					Name:     rbName,
				},
			}

			err = r.Create(ctx, &rb)
			if err != nil {
				return err
			}
		} else {
			rb.Subjects = subjects
			err = r.Update(ctx, &rb)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
