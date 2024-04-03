package keycloak_test

import (
	context "context"

	"testing"

	gocloak "github.com/Nerzal/gocloak/v13"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gomock "github.com/golang/mock/gomock"
)

func TestPutUser_simple(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	subject := gocloak.User{
		ID:        gocloak.StringP("fuuser-id"),
		Username:  gocloak.StringP("fuuser"),
		FirstName: gocloak.StringP("Fuu"),
		LastName:  gocloak.StringP("Ser"),
		Email:     gocloak.StringP("fuuser@email.com"),
		Attributes: &map[string][]string{
			KeycloakDefaultOrganizationRef: {"foo"},
		},
	}

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client: mKeycloak,
	}
	mockLogin(mKeycloak, c)
	mockGetUsers(mKeycloak, c, "fuuser", []*gocloak.User{&subject})
	mockUpdateUser(mKeycloak, c,
		func() gocloak.User {
			updated := subject
			updated.Attributes = &map[string][]string{
				KeycloakDefaultOrganizationRef: {"new-foo"},
			}
			return updated
		}())

	u, err := c.PutUser(context.TODO(), User{Username: "fuuser", DefaultOrganizationRef: "new-foo"})
	require.NoError(t, err)
	assert.Equal(t, "new-foo", u.DefaultOrganizationRef)

	assert.Equal(t, "fuuser-id", u.ID)
	assert.Equal(t, "fuuser", u.Username)
	assert.Equal(t, "fuuser@email.com", u.Email)
	assert.Equal(t, "Fuu Ser", u.DisplayName())
}

func TestPutUser_notFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Client: mKeycloak,
	}
	mockLogin(mKeycloak, c)
	mockGetUsers(mKeycloak, c, "fuuser", []*gocloak.User{})

	_, err := c.PutUser(context.TODO(), User{Username: "fuuser", DefaultOrganizationRef: "new-foo"})
	notFoundErr := UserNotFoundError{}
	require.ErrorIs(t, err, UserNotFoundError{})
	require.ErrorAs(t, err, &notFoundErr)
	require.Equal(t, "fuuser", notFoundErr.Username)
}
