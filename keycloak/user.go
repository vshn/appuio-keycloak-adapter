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

func (u User) DisplayName() string {
	if u.FirstName == "" {
		return u.LastName
	}
	if u.LastName == "" {
		return u.FirstName
	}

	return u.FirstName + " " + u.LastName
}

func (u User) KeycloakUser() gocloak.User {
	r := gocloak.User{}
	if u.ID != "" {
		r.ID = &u.ID
	}
	if u.Username != "" {
		r.Username = &u.Username
	}
	if u.Email != "" {
		r.Email = &u.Email
	}
	if u.FirstName != "" {
		r.FirstName = &u.FirstName
	}
	if u.LastName != "" {
		r.LastName = &u.LastName
	}

	if u.DefaultOrganizationRef != "" {
		attr := make(map[string][]string)
		attr[KeycloakDefaultOrganizationRef] = []string{u.DefaultOrganizationRef}
		r.Attributes = &attr
	}

	return r
}

func (u User) overlay(uo User) User {
	r := u
	if uo.ID != "" {
		r.ID = uo.ID
	}
	if uo.Username != "" {
		r.Username = uo.Username
	}
	if uo.Email != "" {
		r.Email = uo.Email
	}
	if uo.FirstName != "" {
		r.FirstName = uo.FirstName
	}
	if uo.LastName != "" {
		r.LastName = uo.LastName
	}

	if uo.DefaultOrganizationRef != "" {
		r.DefaultOrganizationRef = uo.DefaultOrganizationRef
	}
	return r
}
