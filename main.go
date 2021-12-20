/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/engine"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mholt/archiver/v3"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"oras.land/oras-go/pkg/content"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/fluxcd/pkg/ssa"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/cli"
	"cattle.io/kevi/controllers"
	"cattle.io/kevi/pkg/fetcher"
	"cattle.io/kevi/pkg/install"
	"cattle.io/kevi/pkg/pack"
	"cattle.io/kevi/pkg/webhook"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

const (
	defaultName      = "kevi"
	defaultNamespace = "kevi-system"
	mwhCertsName     = "kevi-webhook-server-cert"
	mwhName          = "kevi-mutating-webhook-configuration"
	ownerKey         = "kevi.cattle.io"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := cli.Cli().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

func cl() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kevi",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	addInstall(cmd)
	addApply(cmd)
	addDeploy(cmd)
	addPack(cmd)
	addCopy(cmd)
	addManager(cmd)

	return cmd
}

func addDeploy(parent *cobra.Command) {
	var (
		packages  []string
		storePath string

		username  string
		password  string
		insecure  bool
		plainHttp bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "deploy packages to a cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			// registy := args[0]
			//
			// for _, p := range packages {
			// 	fmt.Printf("Unpacking package [%s] to [%s]\n", p, storePath)
			// 	if err := archiver.Unarchive(p, storePath); err != nil {
			// 		return err
			// 	}
			// }
			//
			s, err := pack.NewOci(storePath)
			if err != nil {
				return err
			}
			//
			// ropts := content.RegistryOptions{
			// 	Username:  username,
			// 	Password:  password,
			// 	Insecure:  insecure,
			// 	PlainHTTP: plainHttp,
			// }
			//
			// descs, err := s.CopyAll(ctx, registy, ropts)
			// if err != nil {
			// 	return err
			// }
			//
			// for _, d := range descs {
			// 	fmt.Printf("Copied %v\n", d.Annotations[ocispec.AnnotationRefName])
			// }

			// search for any deployable configs
			if err := s.Walk(func(reference string, desc ocispec.Descriptor) error {
				// TODO: Better way to identify known packages, preferably an annotation

				spl := strings.Split(reference, "/kevi-")
				if len(spl) > 1 {
					fmt.Println("Identified kevi configuration: ", reference)
					rc, err := s.Fetch(ctx, desc)
					if err != nil {
						return err
					}
					defer rc.Close()

					m := ocispec.Manifest{}
					s.Fetch(ctx, m.Layers[0])

					u := new(unstructured.Unstructured)
					if err := json.NewDecoder(rc).Decode(&u); err != nil {
						return err
					}

					fmt.Println(u.GetName())
				}

				return nil
			}); err != nil {
				return err
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&storePath, "store", "s", "./store", "Path to store.")
	f.StringSliceVarP(&packages, "package", "f", []string{}, "Path to archived packages.")
	f.StringVarP(&username, "username", "u", "", "Username to use for an authenticated registry.")
	f.StringVarP(&password, "password", "p", "", "Password to use for an authenticated registry.")
	f.BoolVar(&insecure, "insecure", false, "Toggle insecure mode when connecting to registry.")
	f.BoolVar(&plainHttp, "plain-http", false, "Toggle https enforcement when connecting to registry.")

	parent.AddCommand(cmd)
}

func addInstall(parent *cobra.Command) {
	var (
		storePath string
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "install kevi and it's components",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			registry := args[0]

			reg, err := name.NewRegistry(registry)
			if err != nil {
				return err
			}

			// Ensure we can hit registry
			if _, err := remote.Catalog(ctx, reg); err != nil {
				return err
			}

			img, err := install.Build(ctx)
			if err != nil {
				return err
			}

			imgh, err := img.Digest()
			if err != nil {
				return err
			}

			ref := fmt.Sprintf("kevi/kevi-controller@%s", imgh.String())
			refn, err := name.ParseReference(ref, name.WithDefaultRegistry(reg.Name()))
			if err != nil {
				return err
			}

			fmt.Println("Pushed: ", refn.Name())
			if err := remote.Write(refn, img); err != nil {
				return err
			}

			objs, err := install.Generate(ctx, install.Options{
				Namespace: defaultNamespace,
				Registry:  registry,
				Image:     refn.Name(),
			})
			if err != nil {
				return err
			}

			cfg := ctrl.GetConfigOrDie()
			rm, err := apiutil.NewDynamicRESTMapper(cfg)
			kc, err := client.New(cfg, client.Options{Mapper: rm})
			poller := polling.NewStatusPoller(kc, rm)
			mgr := ssa.NewResourceManager(kc, poller, ssa.Owner{
				Field: defaultName,
				Group: ownerKey,
			})
			if err != nil {
				return err
			}

			// TODO: Stupid hack to ensure namespace exists first since technically this is a server side apply with validation
			uns := hack()
			nss, err := mgr.ApplyAll(ctx, []*unstructured.Unstructured{uns}, ssa.ApplyOptions{})
			if err != nil {
				return err
			}
			fmt.Println(nss.String())

			cs, err := mgr.ApplyAll(ctx, objs, ssa.ApplyOptions{
				Force:       false,
				Exclusions:  nil,
				WaitTimeout: 0,
			})
			if err != nil {
				return err
			}
			fmt.Println(cs.String())
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&storePath, "store", "s", "", "Path to store where artifacts are located")

	parent.AddCommand(cmd)
}

func hack() *unstructured.Unstructured {
	ns := corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultNamespace,
		},
	}
	data, err := json.Marshal(ns)
	if err != nil {
		return nil
	}
	u := new(unstructured.Unstructured)
	_ = u.UnmarshalJSON(data)
	return u
}

func addPack(parent *cobra.Command) {
	var (
		store       string
		packages    []string
		archive     bool
		archivePath string
	)

	cmd := &cobra.Command{
		Use:   "pack",
		Short: "package kevis",
		RunE: func(cmd *cobra.Command, args []string) error {
			l := ctrl.Log.WithName("pack")

			opts := zap.Options{
				Development: true,
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			s, err := pack.NewOci(store)
			if err != nil {
				return err
			}

			for _, f := range packages {
				fi, err := os.Open(f)
				if err != nil {
					return err
				}

				reader := yaml.NewYAMLReader(bufio.NewReader(fi))

				var docs [][]byte
				for {
					raw, err := reader.Read()
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}

					docs = append(docs, raw)
				}

				for _, doc := range docs {
					var k v1alpha1.Kevi
					if err := yaml.Unmarshal(doc, &k); err != nil {
						return err
					}

					l.Info("Packaging...", "package", k.Name)
					descs, err := s.Pack(ctx, k)
					if err != nil {
						return err
					}

					l.Info("Finished packaging", "# artifacts", len(descs))
				}
			}

			if archive {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				defer os.Chdir(cwd)
				if err := os.Chdir(store); err != nil {
					return err
				}

				outputPath := filepath.Join(cwd, archivePath)
				if err := archiver.Archive([]string{"."}, outputPath); err != nil {
					return err
				}
				l.Info("Archived and compressed store", "path", archivePath)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&store, "store", "s", "store", "Path to directory where artifacts are stored")
	f.StringSliceVarP(&packages, "package", "p", []string{}, "Paths to package files, can be specified multiple times.")
	f.BoolVarP(&archive, "archive", "a", false, "Toggle archiving the store after processing all packages.")
	f.StringVar(&archivePath, "archive-path", "pkgs.tar.gz", "Path to output archive to, only valid when --archive is true")

	parent.AddCommand(cmd)
}

func addApply(parent *cobra.Command) {
	var (
		files []string
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "apply manifests to a cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			context, cancel := context.WithCancel(context.Background())
			defer cancel()

			var us []*unstructured.Unstructured
			for _, f := range files {
				fi, err := os.Open(f)
				if err != nil {
					return err
				}

				reader := yaml.NewYAMLReader(bufio.NewReader(fi))

				var docs [][]byte
				for {
					raw, err := reader.Read()
					if err == io.EOF {
						break
					}
					if err != nil {
						return err
					}

					docs = append(docs, raw)
				}

				for _, doc := range docs {
					u := new(unstructured.Unstructured)
					if err := yaml.Unmarshal(doc, &u); err != nil {
						return err
					}
					us = append(us, u)
				}
			}

			cfg := ctrl.GetConfigOrDie()
			rm, err := apiutil.NewDynamicRESTMapper(cfg)
			kc, err := client.New(cfg, client.Options{Mapper: rm})
			poller := polling.NewStatusPoller(kc, rm)
			mgr := ssa.NewResourceManager(kc, poller, ssa.Owner{
				Field: defaultName,
				Group: ownerKey,
			})
			if err != nil {
				return err
			}

			cs, err := mgr.ApplyAll(context, us, ssa.ApplyOptions{
				Force:       false,
				Exclusions:  nil,
				WaitTimeout: 0,
			})
			if err != nil {
				return err
			}

			fmt.Println(cs.String())
			return nil
		},
	}

	f := cmd.Flags()
	f.StringSliceVarP(&files, "files", "f", []string{}, "Paths to manifest files, can be specified multiple times.")

	parent.AddCommand(cmd)
}

func addCopy(parent *cobra.Command) {
	var (
		store     string
		username  string
		password  string
		insecure  bool
		plainHttp bool
	)

	cmd := &cobra.Command{
		Use:   "copy",
		Short: "copy kevi store to a target",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			s, err := pack.NewOci(store)
			if err != nil {
				return err
			}

			ropts := content.RegistryOptions{
				Username:  username,
				Password:  password,
				Insecure:  insecure,
				PlainHTTP: plainHttp,
			}

			descs, err := s.CopyAll(ctx, args[0], ropts)
			if err != nil {
				return err
			}

			for _, d := range descs {
				fmt.Printf("Copied %v\n", d.Annotations[ocispec.AnnotationRefName])
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&store, "store", "s", "./store", "Path to store where artifacts are located")
	f.StringVarP(&username, "username", "u", "", "Username to use for an authenticated registry.")
	f.StringVarP(&password, "password", "p", "", "Password to use for an authenticated registry.")
	f.BoolVar(&insecure, "insecure", false, "Toggle insecure mode when connecting to registry.")
	f.BoolVar(&plainHttp, "plain-http", false, "Toggle https enforcement when connecting to registry.")

	parent.AddCommand(cmd)
}

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
				setupLog.Error(err, "failed to initialize gitops-engine")
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
				setupLog.Error(err, "unable to start manager")
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
				setupLog.Error(err, "unable to setup cert rotator")
				os.Exit(1)
			}

			// +kubebuilder:scaffold:builder

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up health check")
				os.Exit(1)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "unable to set up ready check")
				os.Exit(1)
			}

			reconciler := &controllers.KeviReconciler{
				Client:  mgr.GetClient(),
				Scheme:  mgr.GetScheme(),
				Fetcher: registryFetcher,
				Engine:  gengine,
			}
			go initControllers(mgr, reconciler, registry, setupFinished)

			setupLog.Info("starting manager")
			if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
				setupLog.Error(err, "problem running manager")
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

func initControllers(mgr ctrl.Manager, reconciler *controllers.KeviReconciler, registry string, certsCreated chan struct{}) {
	setupLog.Info("waiting for certificate generation/rotation")
	<-certsCreated
	setupLog.Info("certs created")

	setupLog.Info("setting up reconcilers")
	if err := (reconciler).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Kevi")
		os.Exit(1)
	}

	if err := webhook.AddPodRelocatorToManager(mgr, registry); err != nil {
		setupLog.Error(err, "failed to register pod relocator webhook")
		os.Exit(1)
	}
}
