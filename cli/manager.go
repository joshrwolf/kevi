package cli

import (
	"os"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/go-logr/logr"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"oras.land/oras-go/pkg/content"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"cattle.io/kevi/controllers"
	"cattle.io/kevi/pkg/fetcher"
	"cattle.io/kevi/pkg/webhook"
)

func addManager(parent *cobra.Command) {
	var (
		metricsAddr          string
		enableLeaderElection bool
		probeAddr            string
		dev                  bool
		certsDir             string

		registry string
	)

	cmd := &cobra.Command{
		Use:   "manager",
		Short: "run the manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			log := ctrl.Log.WithName("setup")
			opts := zap.Options{
				Development: dev,
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

			cfg := ctrl.GetConfigOrDie()

			c := cache.NewClusterCache(cfg,
				cache.SetLogr(ctrl.Log),
				cache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (info interface{}, cacheManifest bool) {
					gcMark := un.GetAnnotations()[controllers.GCAnnotationMark]
					info = &controllers.GCMark{Mark: un.GetAnnotations()[controllers.GCAnnotationMark]}
					// cache resources that has that mark to improve performance
					cacheManifest = gcMark != ""
					return
				}),
			)

			gengine := engine.NewEngine(cfg, c, engine.WithLogr(ctrl.Log))
			cleanup, err := gengine.Run()
			if err != nil {
				log.Error(err, "failed to initialize gitops-engine")
				os.Exit(1)
			}
			defer cleanup()

			registryFetcher, err := fetcher.NewRegistry(registry, content.RegistryOptions{
				PlainHTTP: true,
				Insecure:  true,
			})
			if err != nil {
				return err
			}

			mgr, err := ctrl.NewManager(cfg, manager.Options{
				Scheme:                 scheme,
				MetricsBindAddress:     metricsAddr,
				HealthProbeBindAddress: probeAddr,
				Port:                   9443,
				CertDir:                certsDir,
				LeaderElection:         enableLeaderElection,
				LeaderElectionID:       "18641b0c.cattle.io",
			})
			if err != nil {
				log.Error(err, "unable to start manager")
				os.Exit(1)
			}

			setupFinished := make(chan struct{})
			cr := &rotator.CertRotator{
				SecretKey: types.NamespacedName{
					Name:      mwhCertsName,
					Namespace: defaultNamespace,
				},
				CAName:         "kevi-ca",
				CAOrganization: "kevi",
				CertDir:        certsDir,
				DNSName:        "kevi-controller-manager.kevi-system.svc",
				IsReady:        setupFinished,
				Webhooks: []rotator.WebhookInfo{
					{Name: mwhName, Type: rotator.Mutating},
				},
			}

			if err := rotator.AddRotator(mgr, cr); err != nil {
				log.Error(err, "unable to setup cert rotator")
				os.Exit(1)
			}

			// +kubebuilder:scaffold:builder

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				log.Error(err, "unable to set up health check")
				os.Exit(1)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				log.Error(err, "unable to set up ready check")
				os.Exit(1)
			}

			reconciler := &controllers.KeviReconciler{
				Client:  mgr.GetClient(),
				Scheme:  mgr.GetScheme(),
				Fetcher: registryFetcher,
				Engine:  gengine,
			}
			go initControllers(mgr, log, reconciler, registry, setupFinished)

			log.Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				log.Error(err, "problem running manager")
				os.Exit(1)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVar(&metricsAddr, "metrics-bind-address", ":9080", "The address the metric endpoint binds to.")
	f.StringVar(&probeAddr, "health-probe-bind-address", ":9081", "The address the probe endpoint binds to.")
	f.StringVar(&certsDir, "certs-dir", "/certs", "Location to source server certificates.")
	f.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	f.BoolVar(&dev, "dev", false, "Toggle development mode (increases logging verbosity).")
	f.StringVar(&registry, "registry", "", "Registry hostname containing package sources.")

	parent.AddCommand(cmd)
}

func initControllers(mgr ctrl.Manager, log logr.Logger, reconciler *controllers.KeviReconciler, registry string, certsCreated chan struct{}) {
	log.Info("waiting for certificate generation/rotation")
	<-certsCreated
	log.Info("certs created")

	log.Info("setting up reconcilers")
	if err := (reconciler).SetupWithManager(mgr); err != nil {
		log.Error(err, "unable to create controller", "controller", "Kevi")
		os.Exit(1)
	}

	if err := webhook.AddPodRelocatorToManager(mgr, registry); err != nil {
		log.Error(err, "failed to register pod relocator webhook")
		os.Exit(1)
	}
}
