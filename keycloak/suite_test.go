package keycloak_test

import (
	. "github.com/vshn/appuio-keycloak-adapter/keycloak"

	gocloak "github.com/Nerzal/gocloak/v10"
	gomock "github.com/golang/mock/gomock"
)

func mockLogin(mgc *MockGoCloak, c Client) {
	mgc.EXPECT().
		LoginAdmin(gomock.Any(), c.Username, c.Password, c.Realm).
		Return(&gocloak.JWT{
			AccessToken: "token",
		}, nil).
		AnyTimes()
}

func mockGetGroups(mgc *MockGoCloak, c Client, groupName string, groups []*gocloak.Group) {
	mgc.EXPECT().
		GetGroups(gomock.Any(), "token", c.Realm, gocloak.GetGroupsParams{
			Search: gocloak.StringP(groupName),
		}).
		Return(groups, nil).
		Times(1)
}

func mockCreateGroup(mgc *MockGoCloak, c Client, groupName, groupID string) {
	mgc.EXPECT().
		CreateGroup(gomock.Any(), "token", c.Realm, gocloak.Group{
			Name: &groupName,
		}).
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
		GetGroupMembers(gomock.Any(), "token", c.Realm, "foo-id", gomock.Any()).
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
