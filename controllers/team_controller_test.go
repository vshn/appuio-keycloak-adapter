package controllers_test

import (
	"context"
	"errors"
	"testing"

	controlv1 "github.com/appuio/control-api/apis/v1"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	. "github.com/vshn/appuio-keycloak-adapter/controllers"
)

func Test_TeamController_Reconcile_Success(t *testing.T) {
	ctx := context.Background()

	c, keyMock, _ := prepareTest(t, barTeam)
	group := keycloak.NewGroup(barTeam.Spec.DisplayName, barTeam.Namespace, barTeam.Name).WithMemberNames("baz", "qux")
	keyMock.EXPECT().
		PutGroup(gomock.Any(), group).
		Return(group, nil).
		Times(1)

	_, err := (&TeamReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: barTeam.Namespace,
			Name:      barTeam.Name,
		},
	})
	require.NoError(t, err)

	reconciledTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: barTeam.Namespace, Name: barTeam.Name}, &reconciledTeam))
	require.Len(t, reconciledTeam.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", reconciledTeam.Finalizers[0], "expected finalizer")
}

func Test_TeamController_Reconcile_Failure(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, barTeam)
	group := keycloak.NewGroup(barTeam.Spec.DisplayName, barTeam.Namespace, barTeam.Name).WithMemberNames("baz", "qux")
	keyMock.EXPECT().
		PutGroup(gomock.Any(), group).
		Return(keycloak.Group{}, errors.New("create failed")).
		Times(1)

	erMock.EXPECT().
		Event(gomock.Any(), "Warning", "UpdateFailed", gomock.Any()).
		Times(1)

	_, err := (&TeamReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
		Recorder: erMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: barTeam.Namespace,
			Name:      barTeam.Name,
		},
	})
	require.Error(t, err)

	reconciledTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: barTeam.Namespace, Name: barTeam.Name}, &reconciledTeam))
	require.Len(t, reconciledTeam.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", reconciledTeam.Finalizers[0], "expected finalizer")
}

func Test_TeamController_Reconcile_Member_Failure(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, barTeam)
	group := keycloak.NewGroup(barTeam.Spec.DisplayName, barTeam.Namespace, barTeam.Name).WithMemberNames("baz", "qux")
	keyMock.EXPECT().
		PutGroup(gomock.Any(), group).
		Return(keycloak.Group{}, &keycloak.MembershipSyncErrors{
			{
				Err:      errors.New("no user 'bar' found"),
				Username: "bar",
				Event:    keycloak.UserAddError,
			},
			{
				Err:      errors.New("permission denied"),
				Username: "foo",
				Event:    keycloak.UserRemoveError,
			},
		}).
		Times(1)

	erMock.EXPECT().
		Eventf(gomock.Any(), "Warning", string(keycloak.UserRemoveError), gomock.Any(), "foo").
		Times(1)
	erMock.EXPECT().
		Eventf(gomock.Any(), "Warning", string(keycloak.UserAddError), gomock.Any(), "bar").
		Times(1)

	_, err := (&TeamReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Recorder: erMock,
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: barTeam.Namespace,
			Name:      barTeam.Name,
		},
	})
	require.NoError(t, err)

	reconciledTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Namespace: barTeam.Namespace, Name: barTeam.Name}, &reconciledTeam))
	require.Len(t, reconciledTeam.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", reconciledTeam.Finalizers[0], "expected finalizer")
}

func Test_TeamController_Reconcile_Delete(t *testing.T) {
	ctx := context.Background()

	team := *barTeam
	now := metav1.Now()
	team.DeletionTimestamp = &now
	team.Finalizers = []string{"keycloak-adapter.vshn.net/finalizer"}

	c, keyMock, _ := prepareTest(t, &team)
	keyMock.EXPECT().
		DeleteGroup(gomock.Any(), team.Namespace, team.Name).
		Return(nil).
		Times(1)

	_, err := (&TeamReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: barTeam.Namespace,
			Name:      barTeam.Name,
		},
	})
	require.NoError(t, err)

	newTeam := controlv1.Team{}
	require.Error(t, c.Get(ctx, types.NamespacedName{
		Namespace: barTeam.Namespace,
		Name:      barTeam.Name,
	}, &newTeam))
}

func Test_TeamController_Reconcile_Delete_Failure(t *testing.T) {
	ctx := context.Background()

	team := *barTeam
	now := metav1.Now()
	team.DeletionTimestamp = &now
	team.Finalizers = []string{"keycloak-adapter.vshn.net/finalizer"}

	c, keyMock, erMock := prepareTest(t, &team)
	keyMock.EXPECT().
		DeleteGroup(gomock.Any(), team.Namespace, team.Name).
		Return(errors.New("Failed to delete")).
		Times(1)

	erMock.EXPECT().
		Event(gomock.Any(), "Warning", "DeletionFailed", gomock.Any()).
		Times(1)

	_, err := (&TeamReconciler{
		Client:   c,
		Recorder: erMock,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Namespace: barTeam.Namespace,
			Name:      barTeam.Name,
		},
	})
	require.Error(t, err)

	newTeam := controlv1.Team{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{
		Namespace: barTeam.Namespace,
		Name:      barTeam.Name,
	}, &newTeam))
}

var barTeam = &controlv1.Team{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: "foo",
		Name:      "bar",
	},
	Spec: controlv1.TeamSpec{
		DisplayName: "Bar Team at Foo Inc.",
		UserRefs: []controlv1.UserRef{
			{
				Name: "baz",
			},
			{
				Name: "qux",
			},
		},
	},
}
