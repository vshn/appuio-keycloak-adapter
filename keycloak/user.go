package keycloak

import "github.com/Nerzal/gocloak/v11"

const (
	// KeycloakDefaultOrganizationRef references the keycloak user attribute.
	// TODO(bastjan) If we add more attributes I'd use struct tags struct{ DefaultOrganizationRef string `kcattr:"appuio.io/default-organization"` }
	KeycloakDefaultOrganizationRef = "appuio.io/default-organization"
)

// User is a representation of a user in keycloak
type User struct {
	ID string
	// Username is the .metadata.name in kubernetes and the .Username field in Keycloak
	Username string

	Email     string
	FirstName string
	LastName  string

	DefaultOrganizationRef string
}

// UserFromKeycloakUser returns a user with attributes mapped from the given keycloak user
func UserFromKeycloakUser(u gocloak.User) User {
	r := User{}
	if u.ID != nil {
		r.ID = *u.ID
	}
	if u.Username != nil {
		r.Username = *u.Username
	}
	if u.Email != nil {
		r.Email = *u.Email
	}
	if u.FirstName != nil {
		r.FirstName = *u.FirstName
	}
	if u.LastName != nil {
		r.LastName = *u.LastName
	}

	if u.Attributes != nil {
		if ref, ok := (*u.Attributes)[KeycloakDefaultOrganizationRef]; ok && len(ref) > 0 {
			r.DefaultOrganizationRef = ref[0]
		}
	}

	return r
}

// DisplayName returns the disply name of this user
func (u User) DisplayName() string {
	if u.FirstName == "" {
		return u.LastName
	}
	if u.LastName == "" {
		return u.FirstName
	}

	return u.FirstName + " " + u.LastName
}

// ApplyTo sets attributes from this user to the given gocloak.User
func (u User) ApplyTo(tu *gocloak.User) {
	if u.ID != "" {
		tu.ID = &u.ID
	}
	if u.Username != "" {
		tu.Username = &u.Username
	}
	if u.Email != "" {
		tu.Email = &u.Email
	}
	if u.FirstName != "" {
		tu.FirstName = &u.FirstName
	}
	if u.LastName != "" {
		tu.LastName = &u.LastName
	}

	if u.DefaultOrganizationRef != "" {
		if tu.Attributes == nil {
			attr := make(map[string][]string)
			tu.Attributes = &attr
		}

		(*tu.Attributes)[KeycloakDefaultOrganizationRef] = []string{u.DefaultOrganizationRef}
	}
}
