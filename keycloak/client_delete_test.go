package keycloak_test

import (
	context "context"
	"strings"

	"testing"

	gocloak "github.com/Nerzal/gocloak/v10"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gomock "github.com/golang/mock/gomock"
)

func TestDeleteGroup_simple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client:   mKeycloak,
		Realm:    "foo",
		Username: "bar",
		Password: "buzz",
	}
	mockLogin(mKeycloak, c)
	mockGetGroups(mKeycloak, c, "foo-gmbh",
		[]*gocloak.Group{
			newGocloakGroup("foo-id", "foo-gmbh"),
		})
	mockDeleteGroup(mKeycloak, c, "foo-id")

	err := c.DeleteGroup(context.TODO(), "foo-gmbh")
	require.NoError(t, err)
}

func newGocloakGroup(id string, path ...string) *gocloak.Group {
	if len(path) == 0 {
		panic("group must have at least one element in path")
	}
	return &gocloak.Group{
		ID:   &id,
		Name: gocloak.StringP(path[len(path)-1]),
		Path: gocloak.StringP("/" + strings.Join(path, "/")),
	}
}
