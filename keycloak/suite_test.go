package keycloak_test

import (
	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gocloak "github.com/Nerzal/gocloak/v11"
	gomock "github.com/golang/mock/gomock"
)

func mockLogin(mgc *MockGoCloak, c Client) {
	mgc.EXPECT().
		LoginAdmin(gomock.Any(), c.Username, c.Password, c.Realm).
		Return(&gocloak.JWT{
			SessionState: "session",
			AccessToken:  "token",
		}, nil).
		AnyTimes()
	mgc.EXPECT().
		LogoutUserSession(gomock.Any(), "token", c.Realm, "session").
		Return(nil).
		AnyTimes()
}

func mockListGroups(mgc *MockGoCloak, c Client, groups []*gocloak.Group) {
	mgc.EXPECT().
		GetGroups(gomock.Any(), "token", c.Realm, gocloak.GetGroupsParams{
			Max: gocloak.IntP(-1),
		}).
		Return(groups, nil).
		Times(1)
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

func mockCreateGroup(mgc *MockGoCloak, c Client, groupName, groupPath, groupID string) {
	kcg := gocloak.Group{
		Name: &groupName,
		Path: &groupPath,
	}
	mgc.EXPECT().
		CreateGroup(gomock.Any(), "token", c.Realm, kcg).
		Return(groupID, nil).
		Times(1)
}
func mockCreateChildGroup(mgc *MockGoCloak, c Client, parentID, groupName, groupPath, groupID string) {
	kcg := gocloak.Group{
		Name: &groupName,
		Path: &groupPath,
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
