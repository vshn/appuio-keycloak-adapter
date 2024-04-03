package keycloak_test

import (
	context "context"
	"fmt"

	"testing"

	gocloak "github.com/Nerzal/gocloak/v13"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gomock "github.com/golang/mock/gomock"
)

func TestListGroups_simple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rst := setupHttpMock()
	defer httpmock.DeactivateAndReset()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client: mKeycloak,
		Host:   "https://example.com",
		Realm:  "myrealm",
	}

	setupChildGroupErrorResponse(c, "foo-id")
	setupChildGroupErrorResponse(c, "bar-id")
	setupChildGroupErrorResponse(c, "parent-id")

	gs := []*gocloak.Group{
		newGocloakGroup("Foo Inc.", "foo-id", "foo-gmbh"),
		newGocloakGroup("Bar Inc.", "bar-id", "bar-gmbh"),
		func() *gocloak.Group {
			g := newGocloakGroup("", "parent-id", "parent-gmbh")
			g.SubGroups = &[]gocloak.Group{*newGocloakGroup("Parent GmbH", "qux-id", "parent-gmbh", "qux-team")}
			return g
		}(),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	mockKeycloakSubgroups(mKeycloak, rst, 3)
	for i, id := range []string{"foo-id", "bar-id", "parent-id", "qux-id"} {
		us := []*gocloak.User{}
		for j := 0; j < i; j++ {
			us = append(us, &gocloak.User{
				ID:       gocloak.StringP(fmt.Sprintf("id-%d", i)),
				Username: gocloak.StringP(fmt.Sprintf("user-%d", i)),
			})
		}
		mockGetGroupMembers(mKeycloak, c, id, us)
	}

	res, err := c.ListGroups(context.TODO())
	require.NoError(t, err)

	assert.Len(t, res, 4)
	assert.Equal(t, "/foo-gmbh", res[0].Path())
	assert.Equal(t, "/bar-gmbh", res[1].Path())
	assert.Equal(t, "/parent-gmbh", res[2].Path())
	assert.Equal(t, "/parent-gmbh/qux-team", res[3].Path())

	assert.Len(t, res[0].Members, 0)
	assert.Len(t, res[1].Members, 1)
	assert.Len(t, res[2].Members, 2)
	assert.Len(t, res[3].Members, 3)

	assert.Equal(t, "user-1", res[1].Members[0].Username)
	assert.Equal(t, "user-2", res[2].Members[1].Username)
}

func TestListGroups_simple_keycloak23(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rst := setupHttpMock()
	defer httpmock.DeactivateAndReset()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client: mKeycloak,
		Host:   "https://example.com",
		Realm:  "myrealm",
	}

	subGroups := &[]gocloak.Group{*newGocloakGroup("Parent GmbH", "qux-id", "parent-gmbh", "qux-team")}
	setupChildGroupResponse(c, "foo-id", make([]gocloak.Group, 0))
	setupChildGroupResponse(c, "bar-id", make([]gocloak.Group, 0))
	setupChildGroupResponse(c, "parent-id", *subGroups)

	gs := []*gocloak.Group{
		newGocloakGroup("Foo Inc.", "foo-id", "foo-gmbh"),
		newGocloakGroup("Bar Inc.", "bar-id", "bar-gmbh"),
		newGocloakGroup("", "parent-id", "parent-gmbh"),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	mockKeycloakSubgroups(mKeycloak, rst, 3)
	for i, id := range []string{"foo-id", "bar-id", "parent-id", "qux-id"} {
		us := []*gocloak.User{}
		for j := 0; j < i; j++ {
			us = append(us, &gocloak.User{
				ID:       gocloak.StringP(fmt.Sprintf("id-%d", i)),
				Username: gocloak.StringP(fmt.Sprintf("user-%d", i)),
			})
		}
		mockGetGroupMembers(mKeycloak, c, id, us)
	}

	res, err := c.ListGroups(context.TODO())
	require.NoError(t, err)

	assert.Len(t, res, 4)
	assert.Equal(t, "/foo-gmbh", res[0].Path())
	assert.Equal(t, "/bar-gmbh", res[1].Path())
	assert.Equal(t, "/parent-gmbh", res[2].Path())
	assert.Equal(t, "/parent-gmbh/qux-team", res[3].Path())

	assert.Len(t, res[0].Members, 0)
	assert.Len(t, res[1].Members, 1)
	assert.Len(t, res[2].Members, 2)
	assert.Len(t, res[3].Members, 3)

	assert.Equal(t, "user-1", res[1].Members[0].Username)
	assert.Equal(t, "user-2", res[2].Members[1].Username)
}

func TestListGroups_RootGroup(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rst := setupHttpMock()
	defer httpmock.DeactivateAndReset()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client:    mKeycloak,
		RootGroup: "root-group",
		Host:      "https://example.com",
		Realm:     "myrealm",
	}

	setupChildGroupErrorResponse(c, "foo-id")
	setupChildGroupErrorResponse(c, "root-group-id")
	gs := []*gocloak.Group{
		newGocloakGroup("Foo Inc.", "foo-id", "foo-gmbh"),
		func() *gocloak.Group {
			g := newGocloakGroup("", "root-group-id", "root-group")
			g.SubGroups = &[]gocloak.Group{
				func() gocloak.Group {
					g := *newGocloakGroup("Foo Inc.", "foo-gmbh-id", "root-group", "foo-gmbh")
					g.SubGroups = &[]gocloak.Group{*newGocloakGroup("Foo Team", "foo-team-id", "root-group", "foo-gmbh", "foo-team")}
					return g
				}()}
			return g
		}(),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	mockKeycloakSubgroups(mKeycloak, rst, 2)
	for _, id := range []string{"foo-gmbh-id", "foo-team-id"} {
		mockGetGroupMembers(mKeycloak, c, id, []*gocloak.User{})
	}

	res, err := c.ListGroups(context.TODO())
	require.NoError(t, err)

	assert.Len(t, res, 2)
	assert.Equal(t, "/foo-gmbh", res[0].Path())
	assert.Equal(t, "/foo-gmbh/foo-team", res[1].Path())
}

func TestListGroups_RootGroup_no_groups_under_root(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rst := setupHttpMock()
	defer httpmock.DeactivateAndReset()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client:    mKeycloak,
		RootGroup: "root-group",
		Host:      "https://example.com",
		Realm:     "myrealm",
	}

	setupChildGroupErrorResponse(c, "foo-id")
	setupChildGroupErrorResponse(c, "root-group-id")

	gs := []*gocloak.Group{
		newGocloakGroup("Foo Inc.", "foo-id", "foo-gmbh"),
		newGocloakGroup("", "root-group-id", "root-group"),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	mockKeycloakSubgroups(mKeycloak, rst, 2)

	res, err := c.ListGroups(context.TODO())
	require.NoError(t, err)
	assert.Len(t, res, 0)
}

func TestListGroups_RootGroup_RootNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rst := setupHttpMock()
	defer httpmock.DeactivateAndReset()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client:    mKeycloak,
		RootGroup: "root-group",
	}

	setupChildGroupErrorResponse(c, "foo-id")

	gs := []*gocloak.Group{
		newGocloakGroup("Foo Inc.", "foo-id", "foo-gmbh"),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	mockKeycloakSubgroups(mKeycloak, rst, 1)

	_, err := c.ListGroups(context.TODO())
	require.Error(t, err)
}
