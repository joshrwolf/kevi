package cli

import (
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/pkg/content"

	"cattle.io/kevi/pkg/pack"
)

func addCopy(parent *cobra.Command) {
	var (
		storePath string
	)

	cmd := &cobra.Command{
		Use:   "relocate",
		Short: "relocate a local store to a registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			l := plog()
			ctx := cmd.Context()
			registry := args[0]

			ropts := content.RegistryOptions{
				PlainHTTP: true,
			}

			l.Info().Msgf("Setting up registry [%s] connection", registry)
			_, err := pingRegistry(ctx, registry, ropts)
			if err != nil {
				return err
			}

			s, err := pack.NewOci(storePath)
			if err != nil {
				return err
			}

			descs, err := s.CopyAll(ctx, registry, ropts)
			if err != nil {
				return err
			}

			for _, desc := range descs {
				l.Info().Msgf("Successfully relocated [%s]", desc.Annotations[ocispec.AnnotationRefName])
			}

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&storePath, "store", "s", "./store", "Path to store.")

	parent.AddCommand(cmd)
}
