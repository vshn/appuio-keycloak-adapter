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

	barOrg := keycloak.NewGroup("bar")
	barOrg.Members = []keycloak.User{
		{Username: "bar", DefaultOrganizationRef: "bar"},
		{Username: "bar3", DefaultOrganizationRef: "bar-mss"},
	}
	barTeam := keycloak.NewGroup("bar", "bar-team")
	barTeam.Members = []keycloak.User{
		{Username: "bar-tm-1"},
		{Username: "bar-tm-2", DefaultOrganizationRef: "bar-outsourcing"},
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return([]keycloak.Group{barOrg, barTeam}, nil).
		Times(1)

	err := (&PeriodicSyncer{
		Client:                     c,
		Recorder:                   erMock,
		Keycloak:                   keyMock,
		SyncClusterRoles:           []string{"import-role", "existing-role"},
		SyncClusterRolesUserPrefix: "appuio#",
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
			Name:     "appuio#bar3",
		},
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "appuio#bar",
		},
	}, rb.Subjects, "create new role")
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "existing-role", Namespace: "bar"}, &rb))
	assert.ElementsMatch(t, []rbacv1.Subject{
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "appuio#bar3",
		},
		{
			Kind:     rbacv1.UserKind,
			APIGroup: rbacv1.GroupName,
			Name:     "appuio#bar",
		},
	}, rb.Subjects, "update exiting role")

	newTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "bar-team", Namespace: "bar"}, &newTeam), "create team under organization")
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "bar-tm-1"},
		{Name: "bar-tm-2"},
	}, newTeam.Spec.UserRefs, "user refs for created team")

	createdUsers := controlv1.UserList{}
	require.NoError(t, c.List(ctx, &createdUsers), "create users")
	comparable := make([]keycloak.User, len(createdUsers.Items))
	for i := range createdUsers.Items {
		comparable[i].Username = createdUsers.Items[i].Name
		comparable[i].DefaultOrganizationRef = createdUsers.Items[i].Spec.Preferences.DefaultOrganizationRef
	}
	assert.ElementsMatch(t, comparable, append(barOrg.Members, barTeam.Members...), "create users found in teams and organizations")
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

	c, keyMock, _ := prepareTest(t, fooOrg, fooMemb, barTeam) // We need to add barMember manually as there is no control API in the tests creating them

	groups := []keycloak.Group{
		keycloak.NewGroup("foo").WithMemberNames("foo", "foo2"),
		keycloak.NewGroup("foo", "bar").WithMemberNames("updated-member-1", "updated-member-2"),
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

	newTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: "foo", Name: "bar"}, &newTeam))
	assert.ElementsMatch(t, []controlv1.UserRef{
		{Name: "baz"},
		{Name: "qux"},
	}, newTeam.Spec.UserRefs)
}

func Test_Sync_Skip_ExistingUsers(t *testing.T) {
	ctx := context.Background()
	subject := controlv1.User{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "appuio.io/v1",
			Kind:       "User",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "existing",
		},
		Spec: controlv1.UserSpec{
			Preferences: controlv1.UserPreferences{
				DefaultOrganizationRef: "current-organization",
			},
		},
	}

	c, keyMock, _ := prepareTest(t, fooOrg, fooMemb, &subject)

	fooGroup := keycloak.NewGroup("foo")
	fooGroup.Members = []keycloak.User{
		{
			Username:               subject.Name,
			DefaultOrganizationRef: "updated-organization",
		},
	}

	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return([]keycloak.Group{fooGroup}, nil).
		Times(1)

	err := (&PeriodicSyncer{
		Client:   c,
		Keycloak: keyMock,
	}).Sync(ctx)
	require.NoError(t, err)

	updatedSubject := controlv1.User{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: subject.Name}, &updatedSubject))
	assert.Equal(t, subject, updatedSubject)
}

func Test_Sync_Skip_UserInMultipleGroups(t *testing.T) {
	ctx := context.Background()
	c, keyMock, _ := prepareTest(t, fooOrg, fooMemb)

	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return([]keycloak.Group{
			keycloak.NewGroup("foo").WithMemberNames("in-multiple-groups"),
			keycloak.NewGroup("foo", "bar").WithMemberNames("in-multiple-groups"),
		}, nil).
		Times(1)

	err := (&PeriodicSyncer{
		Client:   c,
		Keycloak: keyMock,
	}).Sync(ctx)
	require.NoError(t, err)
}
