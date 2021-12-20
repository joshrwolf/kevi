package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"oras.land/oras-go/pkg/content"
	"sigs.k8s.io/cli-utils/pkg/kstatus/polling"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/install"
	"cattle.io/kevi/pkg/pack"
)

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
			l := plog()
			ctx := cmd.Context()
			registry := args[0]

			ropts := content.RegistryOptions{
				Username:  username,
				Password:  password,
				Insecure:  insecure,
				PlainHTTP: true,
			}

			l.Info().Msgf("Setting up registry [%s] connection", registry)
			r, err := pingRegistry(ctx, registry, ropts)
			if err != nil {
				return err
			}
			_ = r

			l.Info().Msgf("Setting up connection to cluster")
			rmgr, err := resourceManager()
			if err != nil {
				return err
			}

			l.Info().Msgf("Installing kevi into cluster")
			cs, err := runInstall(ctx, rmgr, registry, install.Options{
				Namespace: defaultNamespace,
			})
			if err != nil {
				return err
			}
			fmt.Println(cs.String())

			for _, p := range packages {
				fmt.Printf("Unpacking package [%s] to [%s]\n", p, storePath)
				if err := archiver.Unarchive(p, storePath); err != nil {
					return err
				}
			}

			l.Info().Msgf("Loading store [%s]", storePath)
			s, err := pack.NewOci(storePath)
			if err != nil {
				return err
			}

			l.Info().Msgf("Relocating content from [%s] --> [%s]", storePath, registry)
			descs, err := s.CopyAll(ctx, registry, ropts)
			if err != nil {
				return err
			}
			for _, desc := range descs {
				l.Info().Msgf("Successfully relocated [%s]", desc.Annotations[ocispec.AnnotationRefName])
			}

			// search for any deployable configs in the store
			if err := s.Walk(func(reference string, desc ocispec.Descriptor) error {
				// TODO: Better way to identify known packages, preferably an annotation
				spl := strings.Split(reference, "/kevi-")
				if len(spl) > 1 {
					l.Info().Msgf("Found kevi configuration in store [%s], installing...", reference)
					k, err := fetchKevi(ctx, s, reference)
					if err != nil {
						return err
					}

					data, err := json.Marshal(k)
					if err != nil {
						return err
					}

					u := new(unstructured.Unstructured)
					if err := u.UnmarshalJSON(data); err != nil {
						return err
					}

					cs, err := rmgr.Apply(ctx, u, ssa.ApplyOptions{})
					if err != nil {
						return err
					}
					fmt.Println(cs)
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

func fetchKevi(ctx context.Context, oci *pack.Oci, reference string) (*v1alpha1.Kevi, error) {
	_, desc, err := oci.Resolve(ctx, reference)
	if err != nil {
		return nil, err
	}

	rc, err := oci.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var m ocispec.Manifest
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		return nil, err
	}

	for _, l := range m.Layers {
		rc, err := oci.Fetch(ctx, l)
		if err != nil {
			return nil, err
		}

		var k v1alpha1.Kevi
		if err := json.NewDecoder(rc).Decode(&k); err != nil {
			rc.Close()
			continue
		}
		rc.Close()
		return &k, nil
	}
	return nil, fmt.Errorf("unable to parse kevi object from %s", reference)
}

func runInstall(ctx context.Context, rmgr *ssa.ResourceManager, registry string, opts install.Options) (*ssa.ChangeSet, error) {
	img, err := install.Build(ctx)
	if err != nil {
		return nil, err
	}

	h, err := img.Digest()
	if err != nil {
		return nil, err
	}

	repon, err := name.NewRepository("kevi/kevi-controller", name.WithDefaultRegistry(registry))
	if err != nil {
		return nil, err
	}

	refn := repon.Digest(h.String())
	fmt.Println("Pushing created image: ", refn.Name())
	if err := remote.Write(refn, img); err != nil {
		return nil, err
	}
	opts.Registry = registry
	opts.Image = refn.Name()

	objs, err := install.Generate(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Ensure namespace is applied first
	for _, obj := range objs {
		if obj.GetKind() == "Namespace" {
			if _, err := rmgr.Apply(ctx, obj, ssa.ApplyOptions{}); err != nil {
				return nil, err
			}
		}
	}

	return rmgr.ApplyAll(ctx, objs, ssa.ApplyOptions{
		Force:       false,
		Exclusions:  nil,
		WaitTimeout: 0,
	})
}

func resourceManager() (*ssa.ResourceManager, error) {
	cfg := ctrl.GetConfigOrDie()

	restMapper, err := apiutil.NewDynamicRESTMapper(cfg)
	if err != nil {
		return nil, err
	}

	kc, err := client.New(cfg, client.Options{Mapper: restMapper})
	if err != nil {
		return nil, err
	}

	poller := polling.NewStatusPoller(kc, restMapper)
	return ssa.NewResourceManager(kc, poller, ssa.Owner{
		Field: "kevi",
		Group: "cattle.io",
	}), nil
}

func pingRegistry(ctx context.Context, registry string, opts content.RegistryOptions) (*content.Registry, error) {
	regn, err := name.NewRegistry(registry)
	if err != nil {
		return nil, err
	}

	if _, err := remote.Catalog(ctx, regn,
		remote.WithAuthFromKeychain(authn.DefaultKeychain),
		remote.WithAuth(authn.FromConfig(authn.AuthConfig{
			Username: opts.Username,
			Password: opts.Password,
		}))); err != nil {
		return nil, err
	}

	return content.NewRegistry(opts)
}
