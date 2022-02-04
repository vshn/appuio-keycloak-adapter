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

type Client struct {
	client gocloak.GoCloak

	realm    string
	username string
	password string
}

func NewClient(host, realm, username, password string) Client {
	return Client{
		client:   gocloak.NewClient(host),
		realm:    realm,
		username: username,
		password: password,
	}
}

func (c Client) PutGroup(ctx context.Context, group Group) (Group, error) {
	res := Group{
		Name: group.Name,
	}

	token, err := c.client.LoginAdmin(ctx, c.username, c.password, c.realm)
	if err != nil {
		return res, fmt.Errorf("failed binding to keycloak: %w", err)
	}

	found, err := c.getGroupByName(ctx, token, group.Name)
	if err != nil {
		return res, fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		// create group
		id, err := c.client.CreateGroup(ctx, token.AccessToken, c.realm, gocloak.Group{
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

	foundMemb, err := c.client.GetGroupMembers(ctx, token.AccessToken, c.realm, *found.ID, gocloak.GetGroupsParams{})
	if err != nil {
		return res, fmt.Errorf("failed finding group: %w", err)
	}

	for _, fm := range foundMemb {
		if !contains(group.Members, *fm.Username) {
			// user is not in group remove it
			err := c.client.DeleteUserFromGroup(ctx, token.AccessToken, c.realm, *fm.ID, *found.ID)
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

func (c Client) getGroupByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.Group, error) {
	// This may return more than one 1 result
	groups, err := c.client.GetGroups(ctx, token.AccessToken, c.realm, gocloak.GetGroupsParams{
		Search: &name,
	})
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, nil
	}

	// TODO this is wrong
	return groups[0], nil
}

func (c Client) addUsersToGroup(ctx context.Context, token *gocloak.JWT, groupID string, usernames []string) ([]string, error) {
	res := []string{}
	for _, uname := range usernames {
		usr, err := c.getUserByName(ctx, token, uname)
		if err != nil || usr == nil {
			return nil, err
		}
		err = c.client.AddUserToGroup(ctx, token.AccessToken, c.realm, *usr.ID, groupID)
		if err != nil {
			return nil, err
		}
		res = append(res, uname)
	}
	return res, nil
}

func (c Client) getUserByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.User, error) {
	// This may return more than one 1 result
	users, err := c.client.GetUsers(ctx, token.AccessToken, c.realm, gocloak.GetUsersParams{
		Username: &name,
	})
	if err != nil {
		return nil, err
	}

	// TODO this is wrong
	return users[0], nil
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
