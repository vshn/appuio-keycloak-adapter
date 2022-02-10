package main

import (
	"context"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/robfig/cron/v3"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/vshn/appuio-keycloak-adapter/controllers"
	"github.com/vshn/appuio-keycloak-adapter/keycloak"

	//+kubebuilder:scaffold:imports
	"time"
)

var (
	// these variables are populated by Goreleaser when releasing
	version = "unknown"
	commit  = "-dirty-"
	date    = time.Now().Format("2006-01-02")

	appName = "appuio-keycloak-adapter"

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(orgv1.AddToScheme(scheme))
	utilruntime.Must(controlv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	metricsAddr := flag.String("metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	enableLeaderElection := flag.Bool("leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	probeAddr := flag.String("health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")

	host := flag.String("keycloak-url", "", "The address of the Keycloak server (E.g. `https://keycloak.example.com`).")
	realm := flag.String("keycloak-realm", "", "The realm to sync the groups to.")
	username := flag.String("keycloak-username", "", "The username to log in to the Keycloak server.")
	password := flag.String("keycloak-password", "", "The password to log in to the Keycloak server.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()

	kc := keycloak.NewClient(*host, *realm, *username, *password)
	mgr, err := setupManager(kc, ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     *metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: *probeAddr,
		LeaderElection:         *enableLeaderElection,
		LeaderElectionID:       "fe04906e.keycloak-adapter.vshn.net",
	})
	if err != nil {
		setupLog.Error(err, "unable to setup manager")
		os.Exit(1)
	}

	c, err := setupSync(ctx, kc, mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to setup sync")
		os.Exit(1)
	}
	c.Start()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
	setupLog.Info("stopping..")
	<-c.Stop().Done()
}

func setupManager(kc controllers.KeycloakClient, opt ctrl.Options) (ctrl.Manager, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opt)
	if err != nil {
		return nil, err
	}

	if err = (&controllers.OrganizationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Keycloak: kc,
	}).SetupWithManager(mgr); err != nil {
		return nil, err
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, err
	}
	return mgr, err
}

func setupSync(ctx context.Context, keyC keycloak.Client, kubeC client.Client) (*cron.Cron, error) {
	syncLog := ctrl.Log.WithName("sync")
	c := cron.New()
	_, err := c.AddFunc("@every 10s", func() {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		gs, err := keyC.ListGroups(ctx)
		if err != nil {
			syncLog.Error(err, "error listing Keycloak groups")
			return
		}
		syncLog.WithValues("groups", gs).Info("got groups")

		orgs := orgv1.OrganizationList{}
		err = kubeC.List(ctx, &orgs)
		if err != nil {
			syncLog.Error(err, "error listing organizations")
			return
		}
		syncLog.WithValues("orgs", orgs).Info("got organizations")

		orgMap := map[string]struct{}{}
		for _, o := range orgs.Items {
			orgMap[o.Name] = struct{}{}
		}

		for _, g := range gs {
			if _, ok := orgMap[g.Name]; !ok {
				syncLog.WithValues("g", g).Info("creating org")
				org := orgv1.Organization{
					ObjectMeta: metav1.ObjectMeta{
						Name: g.Name,
					},
					Spec: orgv1.OrganizationSpec{
						DisplayName: g.Name,
					},
				}
				err := kubeC.Create(ctx, &org)
				if err != nil {
					syncLog.WithValues("org", org.Name).Error(err, "failed to create organization")
					continue
				}

				orgMemb := controlv1.OrganizationMembers{}
				err = kubeC.Get(ctx, types.NamespacedName{
					Namespace: org.Name,
					Name:      "members",
				}, &orgMemb)
				if err != nil {
					syncLog.WithValues("org", org.Name).Error(err, "failed to get organization members")
					continue
				}
				orgMemb.Spec.UserRefs = make([]controlv1.UserRef, len(g.Members))
				for i, m := range g.Members {
					orgMemb.Spec.UserRefs[i] = controlv1.UserRef{Name: m}
				}
				err = kubeC.Update(ctx, &orgMemb)
				if err != nil {
					syncLog.WithValues("org", org.Name).Error(err, "failed to update organization members")
					continue
				}
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}
