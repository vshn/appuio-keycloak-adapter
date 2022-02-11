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
	log := log.FromContext(ctx)

	gs, err := r.Keycloak.ListGroups(ctx)
	if err != nil {
		log.Error(err, "error listing Keycloak groups")
		return err
	}

	orgMap, err := r.getOrganiztionMap(ctx)
	if err != nil {
		log.Error(err, "error listing Organizations")
		return err
	}

	var groupErr error
	recordError := func(descr string, g keycloak.Group, err error) {
		if groupErr == nil {
			groupErr = errors.New("")
		}
		log.WithValues("group", g).Error(err, descr)
		groupErr = fmt.Errorf("%w\n%s: %s\n    %s", groupErr, g.Name, descr, err.Error())
	}
	for _, g := range gs {
		org, ok := orgMap[g.Name]
		if !ok {
			log.V(1).WithValues("group", g).Info("creating organization")
			org, err = r.startImportOrganizationFromGroup(ctx, g)
			if err != nil {
				recordError("failed to start organization import", g, err)
				continue
			}
		}
		if org.Annotations[orgImportAnnot] == "true" {
			log.V(1).WithValues("group", g).Info("updating organization members")
			err := r.updateOrganizationMembersFromGroup(ctx, g)
			if err != nil {
				recordError("failed to update organization members", g, err)
				continue
			}
			err = r.finishImportOrganizationFromGroup(ctx, org)
			if err != nil {
				recordError("failed to finish organization import", g, err)
				continue
			}
		}
	}
	if groupErr != nil {
		return fmt.Errorf("partial sync failure:\n%w", groupErr)
	}
	return nil
}

func (r *OrganizationReconciler) getOrganiztionMap(ctx context.Context) (map[string]*orgv1.Organization, error) {
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
