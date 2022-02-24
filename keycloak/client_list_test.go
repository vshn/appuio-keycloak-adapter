package keycloak_test

import (
	context "context"
	"fmt"

	"testing"

	gocloak "github.com/Nerzal/gocloak/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gomock "github.com/golang/mock/gomock"
)

func TestListGroups_simple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client:   mKeycloak,
		Realm:    "foo",
		Username: "bar",
		Password: "buzz",
	}

	gs := []*gocloak.Group{
		newGocloakGroup("foo-id", "foo-gmbh"),
		newGocloakGroup("bar-id", "bar-gmbh"),
		func() *gocloak.Group {
			g := newGocloakGroup("parent-id", "parent-gmbh")
			g.SubGroups = &[]gocloak.Group{*newGocloakGroup("qux-id", "parent-gmbh", "qux-team")}
			return g
		}(),
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
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

	assert.Equal(t, "user-1", res[1].Members[0])
	assert.Equal(t, "user-2", res[2].Members[1])
}
