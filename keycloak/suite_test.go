package keycloak_test

import (
	"strings"

	"github.com/go-resty/resty/v2"
	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gocloak "github.com/Nerzal/gocloak/v13"
	gomock "github.com/golang/mock/gomock"
	"github.com/jarcoal/httpmock"
)

func mockLogin(mgc *MockGoCloak, c Client) {
	mgc.EXPECT().
		LoginAdmin(gomock.Any(), c.Username, c.Password, c.Realm).
		Return(&gocloak.JWT{
			SessionState: "session",
			AccessToken:  "token",
			RefreshToken: "refresh",
		}, nil).
		AnyTimes()
	mgc.EXPECT().
		LogoutPublicClient(gomock.Any(), "admin-cli", c.Realm, "token", "refresh").
		Return(nil).
		AnyTimes()
}

func mockListGroups(mgc *MockGoCloak, c Client, groups []*gocloak.Group) {
	mgc.EXPECT().
		GetGroups(gomock.Any(), "token", c.Realm, gocloak.GetGroupsParams{
			Max:                 gocloak.IntP(-1),
			BriefRepresentation: gocloak.BoolP(false),
		}).
		Return(groups, nil).
		Times(1)
}

func mockKeycloakSubgroups(mgc *MockGoCloak, rst *resty.Client, times int) {
	mgc.EXPECT().
		GetRequestWithBearerAuth(gomock.Any(), "token").
		Return(rst.NewRequest()).
		Times(times)
}

func mockGetGroups(mgc *MockGoCloak, c Client, groupName string, groups []*gocloak.Group) {
	mgc.EXPECT().
		GetGroups(gomock.Any(), "token", c.Realm, gocloak.GetGroupsParams{
			Max:    gocloak.IntP(-1),
			Search: gocloak.StringP(groupName),
		}).
		Return(groups, nil).
		Times(1)
}

func mockCreateGroup(mgc *MockGoCloak, c Client, groupName, groupDisplayName, groupPath, groupID string) {
	var attributes *map[string][]string
	if groupDisplayName != "" {
		attrMap := make(map[string][]string)
		attrMap["displayName"] = []string{groupDisplayName}
		attributes = &attrMap
	}
	kcg := gocloak.Group{
		Name:       &groupName,
		Path:       &groupPath,
		Attributes: attributes,
	}
	mgc.EXPECT().
		CreateGroup(gomock.Any(), "token", c.Realm, kcg).
		Return(groupID, nil).
		Times(1)
}
func mockCreateChildGroup(mgc *MockGoCloak, c Client, parentID, groupName, groupDisplayName, groupPath, groupID string) {
	var attributes *map[string][]string
	if groupDisplayName != "" {
		attrMap := make(map[string][]string)
		attrMap["displayName"] = []string{groupDisplayName}
		attributes = &attrMap
	}
	kcg := gocloak.Group{
		Name:       &groupName,
		Path:       &groupPath,
		Attributes: attributes,
	}
	mgc.EXPECT().
		CreateChildGroup(gomock.Any(), "token", c.Realm, parentID, kcg).
		Return(groupID, nil).
		Times(1)
}
func mockDeleteGroup(mgc *MockGoCloak, c Client, groupID string) {
	mgc.EXPECT().
		DeleteGroup(gomock.Any(), "token", c.Realm, groupID).
		Return(nil).
		Times(1)
}

func mockGetGroupMembers(mgc *MockGoCloak, c Client, groupID string, users []*gocloak.User) {
	mgc.EXPECT().
		GetGroupMembers(gomock.Any(), "token", c.Realm, groupID, gomock.Any()).
		Return(users, nil).
		Times(1)
}

func mockGetUser(mgc *MockGoCloak, c Client, userName, userID string) {
	mockGetUsers(mgc, c, userName, []*gocloak.User{
		{
			ID:       gocloak.StringP(userID),
			Username: gocloak.StringP(userName),
		},
	})
}
func mockGetUsers(mgc *MockGoCloak, c Client, userName string, users []*gocloak.User) {
	mgc.EXPECT().
		GetUsers(gomock.Any(), "token", c.Realm, gocloak.GetUsersParams{
			Username: gocloak.StringP(userName),
			Max:      gocloak.IntP(-1),
		}).
		Return(users, nil).
		Times(1)
}

func mockAddUser(mgc *MockGoCloak, c Client, userID, groupID string) {
	mgc.EXPECT().
		AddUserToGroup(gomock.Any(), "token", c.Realm, userID, groupID).
		Return(nil).
		Times(1)
}

func mockRemoveUser(mgc *MockGoCloak, c Client, userID, groupID string) {
	mgc.EXPECT().
		DeleteUserFromGroup(gomock.Any(), "token", c.Realm, userID, groupID).
		Return(nil).
		Times(1)
}

func mockUpdateUser(mgc *MockGoCloak, c Client, user gocloak.User) {
	mgc.EXPECT().
		UpdateUser(gomock.Any(), "token", c.Realm, user).
		Return(nil).
		Times(1)
}

func mockGetServerInfo(mgc *MockGoCloak, version string) {
	mgc.EXPECT().
		GetServerInfo(gomock.Any(), "token").
		Return(&gocloak.ServerInfoRepresentation{
			SystemInfo: &gocloak.SystemInfoRepresentation{
				Version: &version,
			},
		}, nil).
		Times(1)
}

func newGocloakGroup(displayName string, id string, path ...string) *gocloak.Group {
	if len(path) == 0 {
		panic("group must have at least one element in path")
	}
	var attributes *map[string][]string
	if displayName != "" {
		attrMap := make(map[string][]string)
		attrMap["displayName"] = []string{displayName}
		attributes = &attrMap
	}
	return &gocloak.Group{
		ID:         &id,
		Name:       gocloak.StringP(path[len(path)-1]),
		Path:       gocloak.StringP("/" + strings.Join(path, "/")),
		Attributes: attributes,
	}
}

func setupHttpMock() *resty.Client {
	rst := resty.New()
	httpmock.ActivateNonDefault(rst.GetClient())
	return rst
}

func setupChildGroupErrorResponse(c Client, groupID string) {
	httpmock.RegisterResponder("GET", getChildGroupUrl(c.Host, c.Realm, groupID), httpmock.NewStringResponder(404, ""))
}

func setupChildGroupResponse(c Client, groupID string, childGroups []gocloak.Group) {
	httpmock.RegisterResponder("GET", getChildGroupUrl(c.Host, c.Realm, groupID), httpmock.NewJsonResponderOrPanic(200, childGroups))
}

func getChildGroupUrl(host, realm, groupID string) string {
	return strings.Join([]string{host, "admin", "realms", realm, "groups", groupID, "children"}, "/")
}
