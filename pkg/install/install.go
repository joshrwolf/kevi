package install

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"text/template"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Cheat quite a bit here, TODO: properly template this without so much work on the frontend
//go:embed old-generated.yaml
var gen string

//go:embed distroless-nonroot-layer.gz
var distroless []byte

type Options struct {
	Namespace string
	Registry  string
	Image     string
}

func MakeDefaultOptions() Options {
	return Options{
		Registry: "ghcr.io",
	}
}

func Generate(ctx context.Context, opts Options) ([]*unstructured.Unstructured, error) {
	t, err := template.New("tmpl").Parse(gen)
	if err != nil {
		return nil, err
	}

	var templated bytes.Buffer
	w := bufio.NewWriter(&templated)
	if err := t.Execute(w, opts); err != nil {
		return nil, err
	}

	if err := w.Flush(); err != nil {
		return nil, err
	}

	return kube.SplitYAML(templated.Bytes())
}

// Build will build a FROM scratch image containing the executed binary
// TODO: Should we keep distroless:scratch as the base? we could embed the layer (777k) into the binary...
// 		 Benefit with distroless:scratch is we get CA's
func Build(ctx context.Context) (v1.Image, error) {
	epath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	e, err := os.Open(epath)
	if err != nil {
		return nil, err
	}

	tmpdir, err := os.MkdirTemp("", "kevi")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpdir)

	f, err := os.Create(filepath.Join(tmpdir, "kevi"))
	if err != nil {
		return nil, err
	}

	fn := func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewBuffer(distroless)), nil
	}
	dsLayer, err := tarball.LayerFromOpener(fn)
	if err != nil {
		return nil, err
	}

	if _, err := io.Copy(f, e); err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := e.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(f.Name(), os.ModePerm); err != nil {
		return nil, err
	}

	l, err := layer(tmpdir)
	if err != nil {
		return nil, err
	}

	base := mutate.MediaType(empty.Image, ocispec.MediaTypeImageManifest)
	base, err = mutate.ConfigFile(base, &v1.ConfigFile{
		OS:           goruntime.GOOS,
		Architecture: goruntime.GOARCH,
		Author:       "kevi",
		Container:    "kevi-controller",
		Config: v1.Config{
			Env:        []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"},
			WorkingDir: "/",
			User:       "65532:65532",
			Entrypoint: []string{"/kevi", "manager"},
		},
	})
	if err != nil {
		return nil, err
	}

	dbase, err := mutate.Append(base, mutate.Addendum{
		Layer:     dsLayer,
		MediaType: ocispec.MediaTypeImageLayerGzip,
	})

	return mutate.Append(dbase, mutate.Addendum{
		Layer:     l,
		MediaType: ocispec.MediaTypeImageLayerGzip,
	})
}

func layer(root string) (v1.Layer, error) {
	var b bytes.Buffer
	gw := gzip.NewWriter(&b)
	tw := tar.NewWriter(gw)

	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		fi, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !d.IsDir() {
			data, err := os.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, data); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gw.Close(); err != nil {
		return nil, err
	}

	return tarball.LayerFromReader(&b)
}
