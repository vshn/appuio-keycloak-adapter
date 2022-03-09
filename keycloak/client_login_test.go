package keycloak_test

import (
	"context"
	"testing"

	gocloak "github.com/Nerzal/gocloak/v11"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	. "github.com/vshn/appuio-keycloak-adapter/keycloak"
)

func TestLogin(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Realm:  "target-realm",
		Client: mKeycloak,
	}

	mKeycloak.EXPECT().
		LoginAdmin(gomock.Any(), c.Username, c.Password, "target-realm").
		Return(&gocloak.JWT{
			SessionState: "session",
			AccessToken:  "token",
		}, nil).
		AnyTimes()
	mKeycloak.EXPECT().
		LogoutUserSession(gomock.Any(), "token", "target-realm", "session").
		Return(nil).
		AnyTimes()

	mockListGroups(mKeycloak, c, []*gocloak.Group{})

	_, err := c.ListGroups(context.Background())
	require.NoError(t, err)
}

func TestLogin_WithLoginRealm(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mKeycloak := NewMockGoCloak(ctrl)
	c := Client{
		Realm:      "target-realm",
		LoginRealm: "login-realm",
		Client:     mKeycloak,
	}

	mKeycloak.EXPECT().
		LoginAdmin(gomock.Any(), c.Username, c.Password, "login-realm").
		Return(&gocloak.JWT{
			SessionState: "session",
			AccessToken:  "token",
		}, nil).
		AnyTimes()
	mKeycloak.EXPECT().
		LogoutUserSession(gomock.Any(), "token", "login-realm", "session").
		Return(nil).
		AnyTimes()

	mockListGroups(mKeycloak, c, []*gocloak.Group{})

	_, err := c.ListGroups(context.Background())
	require.NoError(t, err)
}
