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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func Test_Sync_Success(t *testing.T) {
	ctx := context.Background()

	c, keyMock := prepareTest(t, fooOrg, fooMemb, &controlv1.OrganizationMembers{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "members",
			Namespace: "bar",
		},
	}) // We need to add barMember manually as there is no control API in the tests creating them

	groups := []keycloak.Group{
		{Name: "bar", Members: []string{"bar", "bar3"}},
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)

	err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
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

}

func Test_Sync_Fail_Update(t *testing.T) {
	ctx := context.Background()

	c, keyMock := prepareTest(t, fooOrg, &controlv1.OrganizationMembers{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "members",
			Namespace: "bar",
		},
	})
	// By not adding buzzMember manually we simulate an error while updating the members resource

	groups := []keycloak.Group{
		{Name: "buzz", Members: []string{"buzz1", "buzz"}},
		{Name: "bar", Members: []string{"bar", "bar3"}},
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)

	err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
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

	c, keyMock := prepareTest(t, fooOrg, fooMemb) // We need to add barMember manually as there is no control API in the tests creating them

	groups := []keycloak.Group{
		{Name: "foo", Members: []string{"foo", "foo2"}},
	}
	keyMock.EXPECT().
		ListGroups(gomock.Any()).
		Return(groups, nil).
		Times(1)

	err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
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
