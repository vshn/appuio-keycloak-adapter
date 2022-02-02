package controllers_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	mock "github.com/vshn/appuio-keycloak-adapter/controllers/mock"
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

func Test_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(orgv1.AddToScheme(scheme))
	utilruntime.Must(controlv1.AddToScheme(scheme))

	tcs := map[string]struct {
		orgName string

		kubeState []client.Object

		group    KeycloakGroup
		errCheck func(err error) bool
	}{
		"GivenNormal_ThenSuccess": {
			orgName:   "foo",
			kubeState: []client.Object{fooOrg, fooMemb},
			group: KeycloakGroup{
				Name:    "foo",
				Members: []string{"bar", "bar3"},
			},
		},
	}

	for name, tc := range tcs {
		t.Run(name, func(t *testing.T) {
			c := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tc.kubeState...).
				Build()
			keyMock := mock.NewMockKeycloakPutter(gomock.NewController(t))
			keyMock.EXPECT().
				PutGroup(gomock.Any(), tc.group).
				Return(tc.group, nil).
				Times(1)

			_, err := (&OrganizationReconciler{
				Client:   c,
				Scheme:   &runtime.Scheme{},
				Keycloak: keyMock,
			}).Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name: "foo",
				},
			})
			if tc.errCheck == nil {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.True(t, tc.errCheck(err))
			}

		})
	}
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
				Username: "bar",
			},
			{
				Username: "bar3",
			},
		},
	},
}
