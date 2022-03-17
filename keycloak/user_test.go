package keycloak

import (
	"testing"

	"github.com/Nerzal/gocloak/v11"
	"github.com/stretchr/testify/require"
)

func TestUserFromKeycloakUser(t *testing.T) {
	require.Equal(t, User{}, UserFromKeycloakUser(gocloak.User{}))

	require.Equal(t, User{
		ID:                     "ID",
		Username:               "Username",
		Email:                  "Email",
		FirstName:              "FirstName",
		LastName:               "LastName",
		DefaultOrganizationRef: "DefaultOrganizationRef",
	}, UserFromKeycloakUser(gocloak.User{
		ID:        gocloak.StringP("ID"),
		Username:  gocloak.StringP("Username"),
		FirstName: gocloak.StringP("FirstName"),
		LastName:  gocloak.StringP("LastName"),
		Email:     gocloak.StringP("Email"),
		Attributes: &map[string][]string{
			KeycloakDefaultOrganizationRef: {"DefaultOrganizationRef"},
		},
	}))
}

func TestUser_KeycloakUser(t *testing.T) {
	require.Equal(t, gocloak.User{}, User{}.KeycloakUser())

	require.Equal(t, gocloak.User{
		ID:        gocloak.StringP("ID"),
		Username:  gocloak.StringP("Username"),
		FirstName: gocloak.StringP("FirstName"),
		LastName:  gocloak.StringP("LastName"),
		Email:     gocloak.StringP("Email"),
		Attributes: &map[string][]string{
			KeycloakDefaultOrganizationRef: {"DefaultOrganizationRef"},
		},
	}, User{
		ID:                     "ID",
		Username:               "Username",
		Email:                  "Email",
		FirstName:              "FirstName",
		LastName:               "LastName",
		DefaultOrganizationRef: "DefaultOrganizationRef",
	}.KeycloakUser(),
	)
}

func TestUser_DisplayName(t *testing.T) {
	require.Equal(t, "Foo Bar", User{FirstName: "Foo", LastName: "Bar"}.DisplayName())
}

func TestUser_overlay(t *testing.T) {
	base := User{
		ID:                     "a",
		Username:               "a",
		Email:                  "a",
		FirstName:              "a",
		LastName:               "a",
		DefaultOrganizationRef: "a",
	}
	require.Equal(t, base, base.overlay(User{}))

	overlay := User{DefaultOrganizationRef: "b"}
	overlayed := base
	overlayed.DefaultOrganizationRef = overlay.DefaultOrganizationRef
	require.Equal(t, overlayed, base.overlay(overlay))

	overlay = User{
		ID:                     "b",
		Username:               "b",
		Email:                  "b",
		FirstName:              "b",
		LastName:               "b",
		DefaultOrganizationRef: "b",
	}
	require.Equal(t, overlay, base.overlay(overlay))
}
