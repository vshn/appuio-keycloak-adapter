package controllers

import (
	"context"
	"errors"
	"fmt"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var orgImportAnnot = "keycloak-adapter.vshn.net/importing"

// Sync lists all Keycloak groups in the realm and creates corresponding Organizations if they do not exist
func (r *OrganizationReconciler) Sync(ctx context.Context) error {
	logger := log.FromContext(ctx)

	gs, err := r.Keycloak.ListGroups(ctx)
	if err != nil {
		return fmt.Errorf("cannot list Keycloak groups: %w", err)
	}

	orgMap, err := r.fetchOrganiztionMap(ctx)
	if err != nil {
		return fmt.Errorf("cannot list Organizations: %w", err)
	}

	var groupErr error

	for _, g := range gs {
		org, err := r.syncGroup(ctx, g, orgMap[g.Name])
		if err != nil {
			if groupErr == nil {
				groupErr = errors.New("")
			}
			logger.WithValues("group", g).Error(err, "import of group failed")
			if org != nil {
				r.Recorder.Event(org, "Warning", "ImportFailed", err.Error())
			}
			groupErr = fmt.Errorf("%w\n%s: %s", groupErr, g.Name, err.Error())
		}
	}
	if groupErr != nil {
		return fmt.Errorf("partial sync failure:\n%w", groupErr)
	}
	return nil
}

func (r *OrganizationReconciler) syncGroup(ctx context.Context, g keycloak.Group, org *orgv1.Organization) (*orgv1.Organization, error) {
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
		err = r.finishImportOrganizationFromGroup(ctx, org)
		if err != nil {
			return org, err
		}
	}
	return org, err
}

func (r *OrganizationReconciler) fetchOrganiztionMap(ctx context.Context) (map[string]*orgv1.Organization, error) {
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

func (r *OrganizationReconciler) startImportOrganizationFromGroup(ctx context.Context, group keycloak.Group) (*orgv1.Organization, error) {
	org := &orgv1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: group.Name,
			Annotations: map[string]string{
				orgImportAnnot: "true",
			},
		},
		Spec: orgv1.OrganizationSpec{
			DisplayName: group.Name,
		},
	}
	err := r.Create(ctx, org)
	return org, err
}

func (r *OrganizationReconciler) finishImportOrganizationFromGroup(ctx context.Context, org *orgv1.Organization) error {
	delete(org.Annotations, orgImportAnnot)
	return r.Update(ctx, org)

}

func (r *OrganizationReconciler) updateOrganizationMembersFromGroup(ctx context.Context, group keycloak.Group) error {
	orgMemb := controlv1.OrganizationMembers{}
	err := r.Get(ctx, types.NamespacedName{
		Namespace: group.Name,
		Name:      "members",
	}, &orgMemb)
	if err != nil {
		return err
	}
	orgMemb.Spec.UserRefs = make([]controlv1.UserRef, len(group.Members))
	for i, m := range group.Members {
		orgMemb.Spec.UserRefs[i] = controlv1.UserRef{Name: m}
	}
	return r.Update(ctx, &orgMemb)
}
