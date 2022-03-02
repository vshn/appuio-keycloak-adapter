package keycloak

import (
	"context"
	"fmt"
	"strings"

	"github.com/Nerzal/gocloak/v11"
)

// Group is a representation of a group in keycloak
type Group struct {
	id string

	path []string

	Members []string
}

// NewGroup creates a new group.
func NewGroup(path ...string) Group {
	return Group{path: path}
}

// NewGroupFromPath creates a new group.
func NewGroupFromPath(path string) Group {
	return NewGroup(strings.Split(strings.TrimPrefix(path, "/"), "/")...)
}

// WithMembers returns a copy of the group with given members added.
func (g Group) WithMembers(members ...string) Group {
	g.Members = members
	return g
}

// Path returns the path of the group.
func (g Group) Path() string {
	if len(g.path) == 0 {
		return ""
	}
	return fmt.Sprintf("/%s", strings.Join(g.path, "/"))
}

// PathMembers returns the split path of the group.
func (g Group) PathMembers() []string {
	return g.path
}

// BaseName returns the name of the group.
func (g Group) BaseName() string {
	if len(g.path) == 0 {
		return ""
	}
	return g.path[len(g.path)-1]
}

// MembershipSyncError is a custom error indicating the failure of syncing the membership of a single user.
type MembershipSyncError struct {
	Err      error
	Username string
	Event    ErrEvent
}

func (err MembershipSyncError) Error() string {
	return err.Err.Error()
}

// ErrEvent is the reason this error was thrown.
// It should be short and unique, imagine people writing switch statements to handle them.
type ErrEvent string

// UserAddError indicates that the client was unable to add the user to the group
var UserAddError ErrEvent = "AddUserFailed"

// UserRemoveError indicates that the client was unable to remove the user from the group
var UserRemoveError ErrEvent = "RemoveUserFailed"

// MembershipSyncErrors is a cusom error that can be used to indicate that the client failed to sync one or more memberships.
type MembershipSyncErrors []MembershipSyncError

func (errs *MembershipSyncErrors) Error() string {
	errMsg := ""
	for _, err := range *errs {
		errMsg = fmt.Sprintf("%s\n", err.Error())
	}
	return errMsg
}

//go:generate go run github.com/golang/mock/mockgen -source=$GOFILE -destination=./ZZ_mock_gocloak_test.go -package keycloak_test

// GoCloak is the subset of methods of the humongous gocloak.GoCloak interface that we actually need.
// This keeps the mock at a more reasonable size
type GoCloak interface {
	LoginAdmin(ctx context.Context, username, password, realm string) (*gocloak.JWT, error)
	LogoutUserSession(ctx context.Context, accessToken, realm, session string) error

	CreateGroup(ctx context.Context, accessToken, realm string, group gocloak.Group) (string, error)
	CreateChildGroup(ctx context.Context, accessToken, realm, groupID string, group gocloak.Group) (string, error)
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
	res := NewGroup(group.path...)

	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return res, fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.Client.LogoutUserSession(ctx, token.AccessToken, c.Realm, token.SessionState)

	found, foundMemb, err := c.getGroupAndMembers(ctx, token, group)
	if err != nil {
		return res, fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		created, err := c.createGroup(ctx, token, group)
		if err != nil {
			return res, err
		}
		found = &created
	}

	membErr := MembershipSyncErrors{}

	for _, fm := range foundMemb {
		if !contains(group.Members, *fm.Username) {
			// user is not in group remove it
			err := c.Client.DeleteUserFromGroup(ctx, token.AccessToken, c.Realm, *fm.ID, *found.ID)
			if err != nil {
				membErr = append(membErr, MembershipSyncError{
					Err:      err,
					Username: *fm.Username,
					Event:    UserRemoveError,
				})
				continue
			}
		} else {
			res.Members = append(res.Members, *fm.Username)
		}
	}
	newMemb := diff(group.Members, res.Members)

	addedMemb, addMembErr := c.addUsersToGroup(ctx, token, *found.ID, newMemb)
	res.Members = append(res.Members, addedMemb...)
	if addMembErr != nil {
		membErr = append(membErr, *addMembErr...)
	}

	if len(membErr) > 0 {
		return res, &membErr
	}
	return res, nil
}

func (c Client) createGroup(ctx context.Context, token *gocloak.JWT, group Group) (gocloak.Group, error) {
	toCreate := gocloak.Group{
		Name: gocloak.StringP(group.BaseName()),
		Path: gocloak.StringP(group.Path()),
	}

	if len(group.PathMembers()) == 1 {
		id, err := c.Client.CreateGroup(ctx, token.AccessToken, c.Realm, toCreate)
		toCreate.ID = &id
		return toCreate, err
	}

	p := group.PathMembers()
	parent, err := c.getGroup(ctx, token, NewGroup(p[0:len(p)-1]...))
	if err != nil {
		return toCreate, fmt.Errorf("could not find parent group for %v: %w", group, err)
	}

	id, err := c.Client.CreateChildGroup(ctx, token.AccessToken, c.Realm, *parent.ID, toCreate)
	toCreate.ID = &id
	return toCreate, err
}

// DeleteGroup deletes the Keycloak group by name.
// The method is idempotent and will not do anything if the group does not exits.
func (c Client) DeleteGroup(ctx context.Context, path ...string) error {
	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.Client.LogoutUserSession(ctx, token.AccessToken, c.Realm, token.SessionState)

	found, err := c.getGroup(ctx, token, NewGroup(path...))
	if err != nil {
		return fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		return nil
	}
	return c.Client.DeleteGroup(ctx, token.AccessToken, c.Realm, *found.ID)
}

// ListGroups returns all Keycloak groups in the realm.
// This is potentially very expensive, as it needs to iterate over all groups to get their members.
func (c Client) ListGroups(ctx context.Context) ([]Group, error) {
	token, err := c.Client.LoginAdmin(ctx, c.Username, c.Password, c.Realm)
	if err != nil {
		return nil, fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.Client.LogoutUserSession(ctx, token.AccessToken, c.Realm, token.SessionState)

	groups, err := c.Client.GetGroups(ctx, token.AccessToken, c.Realm, defaultParams)
	if err != nil {
		return nil, err
	}

	res := flatGroups(groups)

	for i, g := range res {
		memb, err := c.Client.GetGroupMembers(ctx, token.AccessToken, c.Realm, g.id, defaultParams)
		if err != nil {
			return res, fmt.Errorf("failed finding groupmembers for group %s: %w", g.BaseName(), err)
		}
		res[i].Members = make([]string, len(memb))
		for j, m := range memb {
			res[i].Members[j] = *m.Username
		}
	}

	return res, nil
}

func (c Client) getGroup(ctx context.Context, token *gocloak.JWT, toSearch Group) (*gocloak.Group, error) {
	if len(toSearch.PathMembers()) == 0 {
		return nil, nil
	}
	// This may return more than one 1 result
	groups, err := c.Client.GetGroups(ctx, token.AccessToken, c.Realm, gocloak.GetGroupsParams{
		Max:    defaultParams.Max,
		Search: gocloak.StringP(toSearch.BaseName()),
	})
	if err != nil {
		return nil, err
	}

	var find func(groups []gocloak.Group) *gocloak.Group
	find = func(groups []gocloak.Group) *gocloak.Group {
		for i := range groups {
			if groups[i].SubGroups != nil {
				if sub := find(*groups[i].SubGroups); sub != nil {
					return sub
				}
			}
			if *groups[i].Name == toSearch.BaseName() && *groups[i].Path == toSearch.Path() {
				return &groups[i]
			}
		}
		return nil
	}

	g := make([]gocloak.Group, len(groups))
	for i := range groups {
		g[i] = *groups[i]
	}
	return find(g), nil
}

func (c Client) getGroupAndMembers(ctx context.Context, token *gocloak.JWT, toFind Group) (*gocloak.Group, []*gocloak.User, error) {
	group, err := c.getGroup(ctx, token, toFind)
	if err != nil || group == nil {
		return group, nil, err
	}

	foundMemb, err := c.Client.GetGroupMembers(ctx, token.AccessToken, c.Realm, *group.ID, defaultParams)
	if err != nil {
		return group, foundMemb, fmt.Errorf("failed finding groupmembers for group %v: %w", toFind, err)
	}
	return group, foundMemb, nil

}

func (c Client) addUsersToGroup(ctx context.Context, token *gocloak.JWT, groupID string, usernames []string) ([]string, *MembershipSyncErrors) {
	res := []string{}
	errs := MembershipSyncErrors{}
	for _, uname := range usernames {
		usr, err := c.getUserByName(ctx, token, uname)
		if err != nil {
			errs = append(errs, MembershipSyncError{
				Err:      err,
				Username: uname,
				Event:    UserAddError,
			})
			continue
		}
		err = c.Client.AddUserToGroup(ctx, token.AccessToken, c.Realm, *usr.ID, groupID)
		if err != nil {
			errs = append(errs, MembershipSyncError{Err: err, Username: uname, Event: UserAddError})
			continue
		}
		res = append(res, uname)
	}
	if len(errs) > 0 {
		return res, &errs
	}
	return res, nil
}

func (c Client) getUserByName(ctx context.Context, token *gocloak.JWT, name string) (*gocloak.User, error) {
	// This may return more than one 1 result
	users, err := c.Client.GetUsers(ctx, token.AccessToken, c.Realm, gocloak.GetUsersParams{
		Max:      defaultParams.Max,
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
	return nil, fmt.Errorf("no user with username %s found", name)
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

func flatGroups(gcp []*gocloak.Group) []Group {
	rootGroups := make([]gocloak.Group, len(gcp))
	for i := range gcp {
		rootGroups[i] = *gcp[i]
	}

	flat := make([]Group, 0)
	var flatten func([]gocloak.Group)
	flatten = func(groups []gocloak.Group) {
		for _, g := range groups {
			group := NewGroupFromPath(*g.Path)
			group.id = *g.ID
			flat = append(flat, group)
			if g.SubGroups != nil {
				flatten(*g.SubGroups)
			}
		}
	}
	flatten(rootGroups)

	return flat
}

var defaultParams = gocloak.GetGroupsParams{
	Max: gocloak.IntP(-1),
}
