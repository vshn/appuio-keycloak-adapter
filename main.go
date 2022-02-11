package main

import (
	"context"
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/go-logr/logr"
	"github.com/robfig/cron/v3"

	orgv1 "github.com/appuio/control-api/apis/organization/v1"
	controlv1 "github.com/appuio/control-api/apis/v1"

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

	crontab := flag.String("sync-schedule", "@every 5m", "A cron style schedule for the organization synchronization interval.")
	timeout := flag.Duration("sync-timeout", 10*time.Second, "The timeout for a single synchronization run.")

	opts := zap.Options{}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()

	mgr, or, err := setupManager(
		keycloak.NewClient(*host, *realm, *username, *password),
		ctrl.Options{
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

	c, err := setupSync(ctx, or, *crontab, *timeout)
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

func setupManager(kc controllers.KeycloakClient, opt ctrl.Options) (ctrl.Manager, *controllers.OrganizationReconciler, error) {
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), opt)
	if err != nil {
		return nil, nil, err
	}
	or := &controllers.OrganizationReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Keycloak: kc,
	}
	if err = or.SetupWithManager(mgr); err != nil {
		return nil, nil, err
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return nil, nil, err
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return nil, nil, err
	}
	return mgr, or, err
}

func setupSync(ctx context.Context, r *controllers.OrganizationReconciler, crontab string, timout time.Duration) (*cron.Cron, error) {
	syncLog := ctrl.Log.WithName("sync")
	c := cron.New()
	_, err := c.AddFunc(crontab, func() {

		sync := func() error {
			rCtx, cancel := context.WithTimeout(ctx, timout)
			rCtx = logr.NewContext(rCtx, syncLog)
			defer cancel()

			return r.Sync(rCtx)
		}

		err := sync()
		if err == nil {
			return
		}

		// Run with exponential backoff
		backoff := 500 * time.Millisecond
		for i := 0; i < 6; i++ {
			syncLog.Error(err, "failed to sync Keycloak groups")

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff *= 2

			err := sync()
			if err == nil {
				return
			}

		}
		syncLog.Info("failed to sync Keycloak groups - giving up")
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}
