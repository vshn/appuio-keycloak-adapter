package keycloak

import (
	"context"
	"fmt"

	"github.com/Nerzal/gocloak/v10"
)

// Group is a representation of a top level group in keycloak
type Group struct {
	Name    string
	Members []string
}

//go:generate go run github.com/golang/mock/mockgen -source=$GOFILE -destination=./ZZ_mock_gocloak_test.go -package keycloak_test

// GoCloak is the subset of methods of the humongous gocloak.GoCloak interface that we actually need.
// This keeps the mock at a more reasonable size
type GoCloak interface {
	LoginAdmin(ctx context.Context, username, password, realm string) (*gocloak.JWT, error)

	CreateGroup(ctx context.Context, accessToken, realm string, group gocloak.Group) (string, error)
	GetGroups(ctx context.Context, accessToken, realm string, params gocloak.GetGroupsParams) ([]*gocloak.Group, error)
	DeleteGroup(ctx context.Context, accessToken, realm, groupID string) error

	GetGroupMembers(ctx context.Context, accessToken, realm, groupID string, params gocloak.GetGroupsParams) ([]*gocloak.User, error)
	GetUsers(ctx context.Context, accessToken, realm string, params gocloak.GetUsersParams) ([]*gocloak.User, error)
	AddUserToGroup(ctx context.Context, token, realm, userID, groupID string) error
	DeleteUserFromGroup(ctx context.Context, token, realm, userID, groupID string) error
}

// Client interacts with the Keycloak API
type Client struct {
	Client GoCloak

	Realm    string
	Username string
	Password string
}

// NewClient creates a new Client
func NewClient(host, realm, username, password string) Client {
	return Client{
		Client:   gocloak.NewClient(host),
		Realm:    realm,
		Username: username,
		Password: password,
	}
}

// PutGroup creates the provided Keycloak group if it does not exist and adjusts the group members accordingly.
// The method is idempotent.
func (c Client) PutGroup(ctx context.Context, group Group) (Group, error) {
	res := Group{
		Name: group.Name,
	}

	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return res, fmt.Errorf("failed binding to keycloak: %w", err)
	}

	found, foundMemb, err := c.getGroupAndMembersByName(ctx, token, group.Name)
	if err != nil {
		return res, fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		id, err := c.Client.CreateGroup(ctx, token.AccessToken, c.Realm, gocloak.Group{
			Name: gocloak.StringP(group.Name),
		})
		if err != nil {
			return res, err
		}
		found = &gocloak.Group{
			ID:   &id,
			Name: gocloak.StringP(group.Name),
		}
	}

	for _, fm := range foundMemb {
		if !contains(group.Members, *fm.Username) {
			// user is not in group remove it
			err := c.Client.DeleteUserFromGroup(ctx, token.AccessToken, c.Realm, *fm.ID, *found.ID)
			if err != nil {
				return res, err
			}
		} else {
			res.Members = append(res.Members, *fm.Username)
		}
	}
	newMemb := diff(group.Members, res.Members)

	addedMemb, err := c.addUsersToGroup(ctx, token, *found.ID, newMemb)
	res.Members = append(res.Members, addedMemb...)
	return res, err
}

// DeleteGroup deletes the Keycloak group by name.
// The method is idempotent and will not do anything if the group does not exits.
func (c Client) DeleteGroup(ctx context.Context, groupName string) error {
	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return fmt.Errorf("failed binding to keycloak: %w", err)
	}
	found, err := c.getGroupByName(ctx, token, groupName)
	if err != nil {
		return fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		return nil
	}
	return c.Client.DeleteGroup(ctx, token.AccessToken, c.Realm, *found.ID)
}

// ListGroups returns all Keycloak groups in the realm.
// This is potentially very expensive, as it needs to iterate over all groups to get its members.
func (c Client) ListGroups(ctx context.Context) ([]Group, error) {
	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return nil, fmt.Errorf("failed binding to keycloak: %w", err)
	}

	groups, err := c.Client.GetGroups(ctx, token.AccessToken, c.Realm, gocloak.GetGroupsParams{})
	if err != nil {
		return nil, err
	}

	res := make([]Group, len(groups))

	for i, g := range groups {
		res[i].Name = *g.Name
		memb, err := c.Client.GetGroupMembers(ctx, token.AccessToken, c.Realm, *g.ID, gocloak.GetGroupsParams{})
		if err != nil {
			return res, fmt.Errorf("failed finding groupmembers for group %s: %w", *g.Name, err)
		}
    res[i].Members = make([]string, len(memb))
    for j, m := range memb {
      res[i].Members[j] = *m.Username
    }
	}

	return res, nil
}

func (c Client) getGroupByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.Group, error) {
	// This may return more than one 1 result
	groups, err := c.Client.GetGroups(ctx, token.AccessToken, c.Realm, gocloak.GetGroupsParams{
		Search: &name,
	})
	if err != nil {
		return nil, err
	}

	for i := range groups {
		if *groups[i].Name == name {
			return groups[i], err
		}
	}
	return nil, nil
}

func (c Client) getGroupAndMembersByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.Group, []*gocloak.User, error) {
	group, err := c.getGroupByName(ctx, token, name)
	if err != nil || group == nil {
		return group, nil, err
	}

	foundMemb, err := c.Client.GetGroupMembers(ctx, token.AccessToken, c.Realm, *group.ID, gocloak.GetGroupsParams{})
	if err != nil {
		return group, foundMemb, fmt.Errorf("failed finding groupmembers for group %s: %w", name, err)
	}
	return group, foundMemb, nil

}

func (c Client) addUsersToGroup(ctx context.Context, token *gocloak.JWT, groupID string, usernames []string) ([]string, error) {
	res := []string{}
	for _, uname := range usernames {
		usr, err := c.getUserByName(ctx, token, uname)
		if err != nil || usr == nil {
			return nil, err
		}
		err = c.Client.AddUserToGroup(ctx, token.AccessToken, c.Realm, *usr.ID, groupID)
		if err != nil {
			return nil, err
		}
		res = append(res, uname)
	}
	return res, nil
}

func (c Client) getUserByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.User, error) {
	// This may return more than one 1 result
	users, err := c.Client.GetUsers(ctx, token.AccessToken, c.Realm, gocloak.GetUsersParams{
		Username: &name,
	})
	if err != nil {
		return nil, err
	}
	for i := range users {
		if *users[i].Username == name {
			return users[i], nil
		}
	}
	return nil, nil
}

func contains(s []string, a string) bool {
	for _, b := range s {
		if a == b {
			return true
		}
	}
	return false
}

// diff returns the elements in `a` that aren't in `b`.
func diff(a, b []string) []string {
	mb := map[string]struct{}{}
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
