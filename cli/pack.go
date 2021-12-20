package cli

import (
	"bufio"
	"io"
	"os"
	"path/filepath"

	"github.com/mholt/archiver/v3"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/pack"
)

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
			l := plog()
			ctx := cmd.Context()

			opts := zap.Options{
				Development: true,
			}

			ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

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

					l.Info().Msgf("Packaging [%s]", k.Name)
					descs, err := s.Pack(ctx, k)
					if err != nil {
						return err
					}

					for _, desc := range descs {
						l.Info().Msgf("Successfully packaged [%s]", desc.Annotations[ocispec.AnnotationRefName])
					}
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
				l.Info().Msgf("Successfully archived and compressed contents to [%s]", archivePath)
			}
			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&store, "store", "s", "store", "Path to directory where artifacts are stored")
	f.StringSliceVarP(&packages, "package", "f", []string{}, "Paths to package files, can be specified multiple times.")
	f.BoolVarP(&archive, "archive", "a", false, "Toggle archiving the store after processing all packages.")
	f.StringVar(&archivePath, "archive-path", "packages.tar.gz", "Path to output archive to, only used when --archive is true")

	parent.AddCommand(cmd)
}
