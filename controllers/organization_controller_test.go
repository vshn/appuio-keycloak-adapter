package controllers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"
	. "github.com/vshn/appuio-keycloak-adapter/controllers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

func Test_OrganizationController_Reconcile_Success(t *testing.T) {
	ctx := context.Background()

	c, keyMock, _ := prepareTest(t, fooOrg, fooMemb)
	group := keycloak.NewGroup("Foo Inc.", "foo").WithMemberNames("bar", "bar3")
	keyMock.EXPECT().
		PutGroup(gomock.Any(), group).
		Return(group, nil).
		Times(1)

	_, err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
	require.Len(t, newOrg.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newOrg.Finalizers[0], "expected finalizer")
	newMemb := controlv1.OrganizationMembers{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "foo"}, &newMemb))
	require.Len(t, newMemb.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newMemb.Finalizers[0], "expected finalizer")
}

func Test_OrganizationController_Reconcile_Failure(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, fooOrg, fooMemb)
	group := keycloak.NewGroup("Foo Inc.", "foo").WithMemberNames("bar", "bar3")
	keyMock.EXPECT().
		PutGroup(gomock.Any(), group).
		Return(keycloak.Group{}, errors.New("create failed")).
		Times(1)

	erMock.EXPECT().
		Event(gomock.Any(), "Warning", "UpdateFailed", gomock.Any()).
		Times(1)

	_, err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Recorder: erMock,
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.Error(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
	require.Len(t, newOrg.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newOrg.Finalizers[0], "expected finalizer")
	newMemb := controlv1.OrganizationMembers{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "foo"}, &newMemb))
	require.Len(t, newMemb.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newMemb.Finalizers[0], "expected finalizer")
}

func Test_OrganizationController_Reconcile_Member_Failure(t *testing.T) {
	ctx := context.Background()

	c, keyMock, erMock := prepareTest(t, fooOrg, fooMemb)
	group := keycloak.NewGroup("Foo Inc.", "foo").WithMemberNames("bar", "bar3")
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
		Times(2)
	erMock.EXPECT().
		Eventf(gomock.Any(), "Warning", string(keycloak.UserAddError), gomock.Any(), "bar").
		Times(2)

	_, err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Recorder: erMock,
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
	require.Len(t, newOrg.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newOrg.Finalizers[0], "expected finalizer")
	newMemb := controlv1.OrganizationMembers{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "members", Namespace: "foo"}, &newMemb))
	require.Len(t, newMemb.Finalizers, 1, "has finalizer")
	assert.Equal(t, "keycloak-adapter.vshn.net/finalizer", newMemb.Finalizers[0], "expected finalizer")
}

func Test_OrganizationController_Reconcile_Delete(t *testing.T) {
	ctx := context.Background()

	org := *fooOrg
	now := metav1.Now()
	org.DeletionTimestamp = &now
	org.Finalizers = []string{"keycloak-adapter.vshn.net/finalizer"}

	c, keyMock, _ := prepareTest(t, &org, fooMemb)
	keyMock.EXPECT().
		DeleteGroup(gomock.Any(), "foo").
		Return(nil).
		Times(1)

	_, err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.Error(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
}

func Test_OrganizationController_Reconcile_Delete_Failure(t *testing.T) {
	ctx := context.Background()

	org := *fooOrg
	now := metav1.Now()
	org.DeletionTimestamp = &now
	org.Finalizers = []string{"keycloak-adapter.vshn.net/finalizer"}

	c, keyMock, erMock := prepareTest(t, &org, fooMemb)
	keyMock.EXPECT().
		DeleteGroup(gomock.Any(), "foo").
		Return(errors.New("Failed to delete")).
		Times(1)

	erMock.EXPECT().
		Event(gomock.Any(), "Warning", "DeletionFailed", gomock.Any()).
		Times(1)

	_, err := (&OrganizationReconciler{
		Client:   c,
		Recorder: erMock,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.Error(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
}

// Reconcile should ignore organizations that are being imported
func Test_OrganizationController_Reconcile_Ignore(t *testing.T) {
	ctx := context.Background()

	org := *fooOrg
	org.Annotations = map[string]string{
		"keycloak-adapter.vshn.net/importing": "true",
	}

	c, keyMock, _ := prepareTest(t, &org, fooMemb)
	_, err := (&OrganizationReconciler{
		Client:   c,
		Scheme:   &runtime.Scheme{},
		Keycloak: keyMock,
	}).Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name: "foo",
		},
	})
	require.NoError(t, err)

	newOrg := orgv1.Organization{}
	require.NoError(t, c.Get(ctx, types.NamespacedName{Name: "foo"}, &newOrg))
}

func prepareTest(t *testing.T, initObjs ...client.Object) (client.WithWatch, *MockKeycloakClient, *MockEventRecorder) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(orgv1.AddToScheme(scheme))
	utilruntime.Must(controlv1.AddToScheme(scheme))

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(initObjs...).
		Build()
	keyMock := NewMockKeycloakClient(gomock.NewController(t))
	eventMock := NewMockEventRecorder(gomock.NewController(t))
	return c, keyMock, eventMock
}

var fooOrg = &orgv1.Organization{
	ObjectMeta: metav1.ObjectMeta{
		Name: "foo",
	},
	Spec: orgv1.OrganizationSpec{
		DisplayName: "Foo Inc.",
	},
}
var fooMemb = &controlv1.OrganizationMembers{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "members",
		Namespace: "foo",
	},
	Spec: controlv1.OrganizationMembersSpec{
		UserRefs: []controlv1.UserRef{
			{
				Name: "bar",
			},
			{
				Name: "bar3",
			},
		},
	},
}
