package keycloak_test

import (
	context "context"

	"testing"

	gocloak "github.com/Nerzal/gocloak/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gomock "github.com/golang/mock/gomock"
)

func TestPutGroup_simple(t *testing.T) {
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
			{
				ID:   gocloak.StringP("foo-id"),
				Name: gocloak.StringP("foo-gmbh"),
			},
		})
	mockGetGroupMembers(mKeycloak, c, "foo-id",
		[]*gocloak.User{
			{
				ID:       gocloak.StringP("1"),
				Username: gocloak.StringP("user"),
			},
		})
	mockGetUser(mKeycloak, c, "user2", "2")
	mockGetUser(mKeycloak, c, "user3", "3")
	mockAddUser(mKeycloak, c, "3", "foo-id")
	mockAddUser(mKeycloak, c, "2", "foo-id")

	g, err := c.PutGroup(context.TODO(), Group{
		Name: "foo-gmbh",
		Members: []string{
			"user", "user2", "user3",
		},
	})
	require.NoError(t, err)
	assert.Len(t, g.Members, 3)
}

func TestPutGroup_new(t *testing.T) {
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
	mockGetGroups(mKeycloak, c, "foo-gmbh", []*gocloak.Group{})
	mockCreateGroup(mKeycloak, c, "foo-gmbh", "foo-id")
	mockGetUser(mKeycloak, c, "user", "1")
	mockAddUser(mKeycloak, c, "1", "foo-id")

	g, err := c.PutGroup(context.TODO(), Group{
		Name: "foo-gmbh",
		Members: []string{
			"user",
		},
	})
	require.NoError(t, err)
	assert.Len(t, g.Members, 1)
}

func TestPutGroup_multiple_matching_groups(t *testing.T) {
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
			{
				ID:   gocloak.StringP("test-id"),
				Name: gocloak.StringP("foo-gmbh-test"),
			},
			{
				ID:   gocloak.StringP("foo-id"),
				Name: gocloak.StringP("foo-gmbh"),
			},
		})
	mockGetGroupMembers(mKeycloak, c, "foo-id",
		[]*gocloak.User{
			{
				ID:       gocloak.StringP("1"),
				Username: gocloak.StringP("user"),
			},
		})
	mockGetUser(mKeycloak, c, "user2", "2")
	mockGetUser(mKeycloak, c, "user3", "3")
	mockAddUser(mKeycloak, c, "3", "foo-id")
	mockAddUser(mKeycloak, c, "2", "foo-id")

	g, err := c.PutGroup(context.TODO(), Group{
		Name: "foo-gmbh",
		Members: []string{
			"user", "user2", "user3",
		},
	})
	require.NoError(t, err)
	assert.Len(t, g.Members, 3)
}
func TestPutGroup_multiple_matching_users(t *testing.T) {
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
			{
				ID:   gocloak.StringP("foo-id"),
				Name: gocloak.StringP("foo-gmbh"),
			},
		})
	mockGetGroupMembers(mKeycloak, c, "foo-id", []*gocloak.User{})
	mockGetUsers(mKeycloak, c, "user", []*gocloak.User{
		{ID: gocloak.StringP("fakeuser"), Username: gocloak.StringP("user-fake")},
		{ID: gocloak.StringP("1"), Username: gocloak.StringP("user")},
	})
	mockAddUser(mKeycloak, c, "1", "foo-id")

	g, err := c.PutGroup(context.TODO(), Group{
		Name:    "foo-gmbh",
		Members: []string{"user"},
	})
	require.NoError(t, err)
	assert.Len(t, g.Members, 1)
}

func TestPutGroup_remove(t *testing.T) {
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
			{
				ID:   gocloak.StringP("foo-id"),
				Name: gocloak.StringP("foo-gmbh"),
			},
		})
	mockGetGroupMembers(mKeycloak, c, "foo-id",
		[]*gocloak.User{
			{
				ID:       gocloak.StringP("1"),
				Username: gocloak.StringP("user"),
			},
			{
				ID:       gocloak.StringP("4"),
				Username: gocloak.StringP("user4"),
			},
		})
	mockGetUser(mKeycloak, c, "user2", "2")
	mockGetUser(mKeycloak, c, "user3", "3")
	mockAddUser(mKeycloak, c, "3", "foo-id")
	mockAddUser(mKeycloak, c, "2", "foo-id")
	mockRemoveUser(mKeycloak, c, "4", "foo-id")

	g, err := c.PutGroup(context.TODO(), Group{
		Name: "foo-gmbh",
		Members: []string{
			"user", "user2", "user3",
		},
	})
	require.NoError(t, err)
	assert.Len(t, g.Members, 3)
}
