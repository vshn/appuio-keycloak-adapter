package keycloak

import (
	"context"
	"fmt"
	"strings"

	"github.com/Nerzal/gocloak/v13"
	"github.com/go-resty/resty/v2"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Group is a representation of a group in keycloak
type Group struct {
	id string

	path []string

	Members []User

	displayName string
}

// NewGroup creates a new group.
func NewGroup(displayName string, path ...string) Group {
	return Group{path: path, displayName: displayName}
}

// NewGroupFromPath creates a new group.
func NewGroupFromPath(displayName string, path string) Group {
	return NewGroup(displayName, strings.Split(strings.TrimPrefix(path, "/"), "/")...)
}

// WithMemberNames returns a copy of the group with given members added.
func (g Group) WithMemberNames(members ...string) Group {
	m := make([]User, len(members))
	for i := range members {
		m[i].Username = members[i]
	}
	g.Members = m
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

func (err MembershipSyncError) Unwrap() error {
	return err.Err
}

// UserNotFoundError indicates a user could not be found.
type UserNotFoundError struct {
	Username string
}

func (err UserNotFoundError) Is(target error) bool {
	_, ok := target.(UserNotFoundError)
	return ok
}

func (err UserNotFoundError) Error() string {
	return fmt.Sprintf("user %q not found", err.Username)
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
	LogoutPublicClient(ctx context.Context, clientID, realm, accessToken, refreshToken string) error

	CreateGroup(ctx context.Context, accessToken, realm string, group gocloak.Group) (string, error)
	CreateChildGroup(ctx context.Context, accessToken, realm, groupID string, group gocloak.Group) (string, error)
	GetGroups(ctx context.Context, accessToken, realm string, params gocloak.GetGroupsParams) ([]*gocloak.Group, error)
	UpdateGroup(ctx context.Context, accessToken, realm string, updatedGroup gocloak.Group) error
	DeleteGroup(ctx context.Context, accessToken, realm, groupID string) error

	GetGroupMembers(ctx context.Context, accessToken, realm, groupID string, params gocloak.GetGroupsParams) ([]*gocloak.User, error)
	GetUsers(ctx context.Context, accessToken, realm string, params gocloak.GetUsersParams) ([]*gocloak.User, error)
	UpdateUser(ctx context.Context, accessToken, realm string, user gocloak.User) error
	AddUserToGroup(ctx context.Context, token, realm, userID, groupID string) error
	DeleteUserFromGroup(ctx context.Context, token, realm, userID, groupID string) error

	GetRequestWithBearerAuth(ctx context.Context, token string) *resty.Request
}

// Client interacts with the Keycloak API
type Client struct {
	Client GoCloak

	Host  string
	Realm string
	// LoginRealm is used for the client to authenticate against keycloak. If not set Realm is used.
	LoginRealm string
	Username   string
	Password   string

	// RootGroup, if set, transparently manages groups under given root group.
	// Searches and puts groups under the given root group and strips the root group from the return values.
	// The root group must exist in Keycloak.
	RootGroup string
}

// NewClient creates a new Client
func NewClient(host, realm, username, password string) Client {
	return Client{
		Client:   gocloak.NewClient(host),
		Realm:    realm,
		Host:     strings.TrimRight(host, "/"),
		Username: username,
		Password: password,
	}
}

// PutGroup creates the provided Keycloak group if it does not exist and adjusts the group members accordingly.
// The method is idempotent.
func (c Client) PutGroup(ctx context.Context, group Group) (Group, error) {
	res := NewGroup(group.displayName, group.path...)
	group = c.prependRoot(group)

	token, err := c.login(ctx)
	if err != nil {
		return res, fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.logout(ctx, token)

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
	} else {
		if getDisplayNameOfGroup(found) != group.displayName {
			found.Attributes = setDisplayName(found.Attributes, group.displayName)
			err := c.updateGroup(ctx, token, *found)
			if err != nil {
				return res, err
			}
		}
	}

	membErr := MembershipSyncErrors{}

	for _, fm := range foundMemb {
		if !containsUsername(group.Members, *fm.Username) {
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
			res.Members = append(res.Members, UserFromKeycloakUser(*fm))
		}
	}
	newMemb := diffByUsername(group.Members, res.Members)

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
		Name:       gocloak.StringP(group.BaseName()),
		Path:       gocloak.StringP(group.Path()),
		Attributes: setDisplayName(nil, group.displayName),
	}

	if len(group.PathMembers()) == 1 {
		id, err := c.Client.CreateGroup(ctx, token.AccessToken, c.Realm, toCreate)
		toCreate.ID = &id
		return toCreate, err
	}

	p := group.PathMembers()
	parent, err := c.getGroup(ctx, token, NewGroup(group.displayName, p[0:len(p)-1]...))
	if err != nil {
		return toCreate, fmt.Errorf("error finding parent group for %v: %w", group, err)
	}
	if parent == nil {
		return toCreate, fmt.Errorf("could not find parent group for %v", group)
	}

	id, err := c.Client.CreateChildGroup(ctx, token.AccessToken, c.Realm, *parent.ID, toCreate)
	toCreate.ID = &id
	return toCreate, err
}

func (c Client) updateGroup(ctx context.Context, token *gocloak.JWT, group gocloak.Group) error {
	err := c.Client.UpdateGroup(ctx, token.AccessToken, c.Realm, group)
	return err
}

// DeleteGroup deletes the Keycloak group by name.
// The method is idempotent and will not do anything if the group does not exits.
func (c Client) DeleteGroup(ctx context.Context, path ...string) error {
	token, err := c.login(ctx)
	if err != nil {
		return fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.logout(ctx, token)

	found, err := c.getGroup(ctx, token, c.prependRoot(NewGroup("", path...)))
	if err != nil {
		return fmt.Errorf("failed finding group: %w", err)
	}
	if found == nil {
		return nil
	}
	return c.Client.DeleteGroup(ctx, token.AccessToken, c.Realm, *found.ID)
}

// ListGroups returns all top-level Keycloak groups in the realm and their direct children.
// More deeply nested children are not returned.
// This is potentially very expensive, as it needs to iterate over all groups to get their members and sub groups.
func (c Client) ListGroups(ctx context.Context) ([]Group, error) {
	token, err := c.login(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.logout(ctx, token)

	groups, err := c.Client.GetGroups(ctx, token.AccessToken, c.Realm, defaultParams)
	if err != nil {
		return nil, err
	}

	for _, g := range groups {
		logger := log.FromContext(ctx)
		subgroups, err := c.getChildGroups(ctx, token, *g.ID)
		if err != nil {
			apiErr, ok := err.(*gocloak.APIError)
			if !ok || apiErr.Code != 404 {
				return nil, fmt.Errorf("failed to fetch sub groups: %w", err)
			}
			logger.Info("Could not fetch sub groups with error 404 - assuming Keycloak 22 API", "error", err, "group", g.Name)
		} else {
			g.SubGroups = &subgroups
		}
	}

	rootGroups := c.filterTreeWithRoot(groups)
	if rootGroups == nil {
		return nil, fmt.Errorf("could not find root group %q", c.RootGroup)
	}

	res := flatGroups(rootGroups)

	for i, g := range res {
		memb, err := c.Client.GetGroupMembers(ctx, token.AccessToken, c.Realm, g.id, defaultParams)
		if err != nil {
			return res, fmt.Errorf("failed finding groupmembers for group %s: %w", g.BaseName(), err)
		}
		res[i].Members = make([]User, len(memb))
		for j, m := range memb {
			res[i].Members[j] = UserFromKeycloakUser(*m)
		}
	}

	return res, nil
}

func (c Client) loginRealm() string {
	if c.LoginRealm != "" {
		return c.LoginRealm
	}
	return c.Realm
}

func (c Client) login(ctx context.Context) (*gocloak.JWT, error) {
	return c.Client.LoginAdmin(ctx, c.Username, c.Password, c.loginRealm())
}

func (c Client) logout(ctx context.Context, token *gocloak.JWT) error {
	// `admin-cli` is the client used when authenticating to the admin API
	return c.Client.LogoutPublicClient(ctx, "admin-cli", c.loginRealm(), token.AccessToken, token.RefreshToken)
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

func (c Client) getChildGroups(ctx context.Context, token *gocloak.JWT, groupID string) ([]gocloak.Group, error) {
	var result []*gocloak.Group
	childGroupsUrl := strings.Join([]string{c.Host, "admin", "realms", c.Realm, "groups", groupID, "children"}, "/")
	resp, err := c.Client.GetRequestWithBearerAuth(ctx, token.AccessToken).
		SetResult(&result).
		Get(childGroupsUrl)

	if err != nil {
		return nil, &gocloak.APIError{
			Code:    0,
			Message: "could not retrieve child groups",
			Type:    gocloak.ParseAPIErrType(err),
		}
	}

	if resp == nil {
		return nil, &gocloak.APIError{
			Message: "empty response",
			Type:    gocloak.ParseAPIErrType(err),
		}
	}

	if resp.IsError() {
		var msg string

		if e, ok := resp.Error().(*gocloak.HTTPErrorResponse); ok && e.NotEmpty() {
			msg = fmt.Sprintf("%s: %s", resp.Status(), e)
		} else {
			msg = resp.Status()
		}

		return nil, &gocloak.APIError{
			Code:    resp.StatusCode(),
			Message: msg,
			Type:    gocloak.ParseAPIErrType(err),
		}
	}

	groupList := make([]gocloak.Group, len(result))

	for i := 0; i < len(result); i++ {
		groupList[i] = *result[i]

	}

	return groupList, nil
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

func (c Client) addUsersToGroup(ctx context.Context, token *gocloak.JWT, groupID string, users []User) ([]User, *MembershipSyncErrors) {
	res := make([]User, 0, len(users))
	errs := MembershipSyncErrors{}
	for _, user := range users {
		usr, err := c.getUserByName(ctx, token, user.Username)
		if err != nil {
			errs = append(errs, MembershipSyncError{
				Err:      err,
				Username: user.Username,
				Event:    UserAddError,
			})
			continue
		}
		err = c.Client.AddUserToGroup(ctx, token.AccessToken, c.Realm, *usr.ID, groupID)
		if err != nil {
			errs = append(errs, MembershipSyncError{Err: err, Username: user.Username, Event: UserAddError})
			continue
		}
		res = append(res, user)
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
	return nil, UserNotFoundError{Username: name}
}

func (c Client) prependRoot(g Group) Group {
	if c.RootGroup != "" {
		g.path = append([]string{c.RootGroup}, g.path...)
	}
	return g
}

func (c Client) filterTreeWithRoot(groups []*gocloak.Group) []gocloak.Group {
	if c.RootGroup == "" {
		rootGroups := make([]gocloak.Group, len(groups))
		for i := range groups {
			rootGroups[i] = *groups[i]
		}
		return rootGroups
	}

	trim := "/" + c.RootGroup
	var trimPath func([]gocloak.Group) []gocloak.Group
	trimPath = func(gs []gocloak.Group) []gocloak.Group {
		for i := range gs {
			g := &gs[i]
			g.Path = gocloak.StringP(strings.TrimPrefix(*g.Path, trim))
			if g.SubGroups != nil {
				sg := trimPath(*g.SubGroups)
				g.SubGroups = &sg
			}
		}
		return gs
	}

	for _, g := range groups {
		if *g.Name == c.RootGroup {
			if g.SubGroups != nil {
				return trimPath(*g.SubGroups)
			}
			return []gocloak.Group{}
		}
	}

	return nil
}

// PutUser updates the given user referenced by its `Username` property.
// An error is returned if a user can't be found.
func (c Client) PutUser(ctx context.Context, user User) (User, error) {
	token, err := c.login(ctx)
	if err != nil {
		return User{}, fmt.Errorf("failed binding to keycloak: %w", err)
	}
	defer c.logout(ctx, token)

	kcUser, err := c.getUserByName(ctx, token, user.Username)
	if err != nil {
		return User{}, fmt.Errorf("failed querying keycloak for user %q: %w", user.Username, err)
	}

	user.ApplyTo(kcUser)
	return UserFromKeycloakUser(*kcUser),
		c.Client.UpdateUser(ctx, token.AccessToken, c.Realm, *kcUser)
}

func containsUsername(s []User, a string) bool {
	for _, b := range s {
		if a == b.Username {
			return true
		}
	}
	return false
}

// diff returns the elements in `a` that aren't in `b`.
func diffByUsername(a, b []User) []User {
	mb := map[string]struct{}{}
	for _, x := range b {
		mb[x.Username] = struct{}{}
	}
	var diff []User
	for _, x := range a {
		if _, found := mb[x.Username]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func flatGroups(gcp []gocloak.Group) []Group {
	flat := make([]Group, 0)
	var flatten func([]gocloak.Group)
	flatten = func(groups []gocloak.Group) {
		for _, g := range groups {
			group := NewGroupFromPath(getDisplayNameOfGroup(&g), *g.Path)
			group.id = *g.ID
			flat = append(flat, group)
			if g.SubGroups != nil {
				flatten(*g.SubGroups)
			}
		}
	}
	flatten(gcp)

	return flat
}

func getDisplayNameOfGroup(group *gocloak.Group) string {
	if group.Attributes != nil {
		displayNames, ok := (*group.Attributes)["displayName"]
		if ok && len(displayNames) > 0 {
			return displayNames[0]
		}
	}
	return ""
}

func setDisplayName(attributes *map[string][]string, displayName string) *map[string][]string {
	if attributes == nil {
		attrMap := make(map[string][]string)
		attributes = &attrMap
	}
	if displayName == "" {
		delete(*attributes, "displayName")
	} else {
		(*attributes)["displayName"] = []string{displayName}
	}
	return attributes
}

var defaultParams = gocloak.GetGroupsParams{
	Max:                 gocloak.IntP(-1),
	BriefRepresentation: gocloak.BoolP(false), // required in order to get attributes when listing groups
}
