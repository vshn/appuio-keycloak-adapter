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
	log.WithValues("groups", gs).Info("got groups")

	orgs := orgv1.OrganizationList{}
	err = r.List(ctx, &orgs)
	if err != nil {
		log.Error(err, "error listing organizations")
		return err
	}
	log.WithValues("orgs", orgs).Info("got organizations")

	orgMap := map[string]struct{}{}
	for _, o := range orgs.Items {
		orgMap[o.Name] = struct{}{}
	}

	for _, g := range gs {
		if _, ok := orgMap[g.Name]; !ok {
			log.WithValues("g", g).Info("creating org")
			org := orgv1.Organization{
				ObjectMeta: metav1.ObjectMeta{
					Name: g.Name,
				},
				Spec: orgv1.OrganizationSpec{
					DisplayName: g.Name,
				},
			}
			err := r.Create(ctx, &org)
			if err != nil {
				log.WithValues("org", org.Name).Error(err, "failed to create organization")
				continue
			}

			orgMemb := controlv1.OrganizationMembers{}
			err = r.Get(ctx, types.NamespacedName{
				Namespace: org.Name,
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
		}
	}
	return nil
}
