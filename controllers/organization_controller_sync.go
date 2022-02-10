package controllers

import (
	"context"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var orgImportAnnot = "keycloak-adapter.vshn.net/importing"

func (r *OrganizationReconciler) Sync(ctx context.Context) error {
	log := log.FromContext(ctx)

	gs, err := r.Keycloak.ListGroups(ctx)
	if err != nil {
		log.Error(err, "error listing Keycloak groups")
		return err
	}

	orgs := orgv1.OrganizationList{}
	err = r.List(ctx, &orgs)
	if err != nil {
		log.Error(err, "error listing organizations")
		return err
	}

	orgMap := map[string]*orgv1.Organization{}
	for i, o := range orgs.Items {
		orgMap[o.Name] = &orgs.Items[i]
	}

	for _, g := range gs {
		org, ok := orgMap[g.Name]
		if !ok {
			log.V(1).WithValues("group", g).Info("creating organization")
			org = &orgv1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name: g.Name,
					Annotations: map[string]string{
						orgImportAnnot: "true",
					},
				},
				Spec: orgv1.OrganizationSpec{
					DisplayName: g.Name,
				},
			}
			err := r.Create(ctx, org)
			if err != nil {
				log.WithValues("org", org.Name).Error(err, "failed to create organization")
				continue
			}
		}
		if org.Annotations[orgImportAnnot] == "true" {
			log.V(1).WithValues("group", g).Info("updating organization members")
			orgMemb := controlv1.OrganizationMembers{}
			err = r.Get(ctx, types.NamespacedName{
				Namespace: g.Name,
				Name:      "members",
			}, &orgMemb)
			if err != nil {
				log.WithValues("org", org.Name).Error(err, "failed to get organization members")
				continue
			}
			orgMemb.Spec.UserRefs = make([]controlv1.UserRef, len(g.Members))
			for i, m := range g.Members {
				orgMemb.Spec.UserRefs[i] = controlv1.UserRef{Name: m}
			}
			err = r.Update(ctx, &orgMemb)
			if err != nil {
				log.WithValues("org", org.Name).Error(err, "failed to update organization members")
				continue
			}

			delete(org.Annotations, orgImportAnnot)
			err = r.Update(ctx, org)
			if err != nil {
				log.WithValues("org", org.Name).Error(err, "failed to update organization")
				continue
			}
		}
	}
	return nil
}
