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
		{
			ID:   gocloak.StringP("foo-id"),
			Name: gocloak.StringP("foo-gmbh"),
		},
		{
			ID:   gocloak.StringP("bar-id"),
			Name: gocloak.StringP("bar-gmbh"),
		},
		{
			ID:   gocloak.StringP("buzz-id"),
			Name: gocloak.StringP("buzz-gmbh"),
		},
	}
	mockLogin(mKeycloak, c)
	mockListGroups(mKeycloak, c, gs)
	for i := range gs {
		us := []*gocloak.User{}
		for j := 0; j < i; j++ {
			us = append(us, &gocloak.User{
				ID:       gocloak.StringP(fmt.Sprintf("id-%d", i)),
				Username: gocloak.StringP(fmt.Sprintf("user-%d", i)),
			})
		}
		mockGetGroupMembers(mKeycloak, c, *gs[i].ID, us)
	}

	res, err := c.ListGroups(context.TODO())
	require.NoError(t, err)

	assert.Len(t, res, 3)
	assert.Len(t, res[0].Members, 0)
	assert.Equal(t, "user-1", res[1].Members[0])
	assert.Equal(t, "user-2", res[2].Members[1])
}
