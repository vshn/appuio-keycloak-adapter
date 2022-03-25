package keycloak

import (
	"testing"
	"time"

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

func TestUser_DisplayName(t *testing.T) {
	require.Equal(t, "Foo Bar", User{FirstName: "Foo", LastName: "Bar"}.DisplayName())
	require.Equal(t, "Foo", User{FirstName: "Foo"}.DisplayName())
	require.Equal(t, "Bar", User{LastName: "Bar"}.DisplayName())
}

func TestUser_ApplyTo(t *testing.T) {
	user := User{
		ID:                     "ID",
		Username:               "Username",
		Email:                  "Email",
		FirstName:              "FirstName",
		LastName:               "LastName",
		DefaultOrganizationRef: "DefaultOrganizationRef",
	}
	expected := baseKeycloakUser()
	expected.ID = &user.ID
	expected.Username = &user.Username
	expected.Email = &user.Email
	expected.FirstName = &user.FirstName
	expected.LastName = &user.LastName
	expected.Attributes = &map[string][]string{
		"example.com/dark-mode":        {"true"},
		KeycloakDefaultOrganizationRef: {"DefaultOrganizationRef"},
	}
	subject := baseKeycloakUser()
	user.ApplyTo(&subject)
	require.Equal(t, expected, subject, "overwrite all attributes")

	subject = baseKeycloakUser()
	subject.Attributes = nil
	user.ApplyTo(&subject)
	require.Equal(t, &map[string][]string{
		KeycloakDefaultOrganizationRef: {"DefaultOrganizationRef"},
	}, subject.Attributes, "create .Attributes if nil")

	subject = baseKeycloakUser()
	User{}.ApplyTo(&subject)
	require.Equal(t, baseKeycloakUser(), subject, "no attributes overridden")
}

func baseKeycloakUser() gocloak.User {
	return gocloak.User{
		ID:               p("base"),
		CreatedTimestamp: p(time.Date(2000, time.January, 2, 3, 3, 3, 0, time.UTC).UnixNano()),
		Username:         p("base"),
		Enabled:          p(true),
		FirstName:        p("base"),
		LastName:         p("base"),
		Email:            p("base"),
		Attributes:       p(map[string][]string{"example.com/dark-mode": {"true"}}),
		RequiredActions:  p([]string{"reset-credentials"}),
		Credentials:      p([]gocloak.CredentialRepresentation{{ID: p("cred-id")}}),
	}
}

func p[T any](a T) *T {
	return &a
}
