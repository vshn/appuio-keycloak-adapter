package controllers_test

import (
	"context"
	"testing"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"

	. "github.com/vshn/appuio-keycloak-adapter/controllers"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_Sync_Success(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, fooOrg, fooMemb,
		// We need to add barMember manually as there is no control API in the tests creating them
		&controlv1.OrganizationMembers{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "members",
				Namespace: "bar",
			},
		},
		// A RoleBinding created by control-api. On import we want to overwrite this
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "existing-role",
				Namespace: "bar",
			},
			Subjects: []rbacv1.Subject{},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "existing-role",
			},
		},
	)

	groups := []keycloak.Group{
		keycloak.NewGroup("bar").WithMemberNames("bar", "bar3"),
		keycloak.NewGroup("bar", "bar-team").WithMemberNames("bar-tm-1", "bar-tm-2"),
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)

	err := (&PeriodicSyncer{
		Client:           c,
		Recorder:         erMock,
		Keycloak:         keyMock,
		SyncClusterRoles: []string{"import-role", "existing-role"},
	}).Sync(ctx)
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "bar"}, &newOrg))
	assert.NotContains(t, newOrg.Annotations, "keycloak-adapter.vshn.net/importing")
	newMemb := controlv1.OrganizationMembers{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "bar"}, &newMemb))
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "bar3"},
		{Name: "bar"},
	}, newMemb.Spec.UserRefs)
	rb := rbacv1.RoleBinding{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "import-role", Namespace: "bar"}, &rb))
	assert.ElementsMatch(t, []rbacv1.Subject{
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "bar3",
		},
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "bar",
		},
	}, rb.Subjects, "create new role")
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "existing-role", Namespace: "bar"}, &rb))
	assert.ElementsMatch(t, []rbacv1.Subject{
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "bar3",
		},
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "bar",
		},
	}, rb.Subjects, "update exiting role")

	newTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "bar-team", Namespace: "bar"}, &newTeam), "create team under organization")
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "bar-tm-1"},
		{Name: "bar-tm-2"},
	}, newTeam.Spec.UserRefs, "user refs for created team")
}

func Test_Sync_Fail_Update(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, fooOrg, &controlv1.OrganizationMembers{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "members",
			Namespace: "bar",
		},
	})
	// By not adding buzzMember manually we simulate an error while updating the members resource

	groups := []keycloak.Group{
		keycloak.NewGroup("buzz").WithMemberNames("buzz1", "buzz"),
		keycloak.NewGroup("bar").WithMemberNames("bar", "bar3"),
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)
	erMock.EXPECT().
		Event(gomock.Any(), "Warning", "ImportFailed", gomock.Any()).
		Times(1)

	err := (&PeriodicSyncer{
		Client:   c,
		Recorder: erMock,
		Keycloak: keyMock,
	}).Sync(ctx)
	assert.Error(t, err)

	newOrg := orgv1.Organization{}
	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "bar"}, &newOrg))
	assert.NotContains(t, newOrg.Annotations, "keycloak-adapter.vshn.net/importing")
	newMemb := controlv1.OrganizationMembers{}
	assert.Error(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "buzz"}, &newMemb))

	assert.NoError(t, c.Get(ctx, types.NamespacedName{Name: "buzz"}, &newOrg))
	assert.Equal(t, "true", newOrg.Annotations["keycloak-adapter.vshn.net/importing"])
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "bar"}, &newMemb))
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "bar3"},
		{Name: "bar"},
	}, newMemb.Spec.UserRefs)

}

func Test_Sync_Skip_Existing(t *testing.T) {
	ctx := context.Background()

	c, keyMock, _ := prepareTest(t, fooOrg, fooMemb) // We need to add barMember manually as there is no control API in the tests creating them

	groups := []keycloak.Group{
		keycloak.NewGroup("foo").WithMemberNames("foo", "foo2"),
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)

	err := (&PeriodicSyncer{
		Client:   c,
		Keycloak: keyMock,
	}).Sync(ctx)
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
	assert.NotContains(t, newOrg.Annotations, "keycloak-adapter.vshn.net/importing")
	newMemb := controlv1.OrganizationMembers{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "foo"}, &newMemb))
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "bar3"},
		{Name: "bar"},
	}, newMemb.Spec.UserRefs)

}
