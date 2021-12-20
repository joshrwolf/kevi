package fetcher

import (
	"context"
	"fmt"
	"path"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rancherfederal/ocil/pkg/consts"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"
	"oras.land/oras-go/pkg/target"

	"cattle.io/kevi/api/v1alpha1"
)

var _ Fetcher = &registry{}

const (
	DefaultRepositoryNamespace = "kevi"
)

type registry struct {
	Hostname string
	store    *content.Registry
}

func NewRegistry(hostname string, opts content.RegistryOptions) (*registry, error) {
	if hostname == "" {
		return nil, fmt.Errorf("registry hostname cannot be empty")
	}

	if _, err := name.NewRegistry(hostname); err != nil {
		return nil, err
	}

	r, err := content.NewRegistry(opts)
	if err != nil {
		return nil, err
	}

	return &registry{
		Hostname: hostname,
		store:    r,
	}, nil
}

func (r *registry) Fetch(ctx context.Context, to target.Target, pkg v1alpha1.KeviSpecPackage) ([]v1.Descriptor, error) {
	var (
		ref    = r.Locate(pkg)
		ldescs []v1.Descriptor
	)

	mt, err := r.contentMediaType(pkg)
	if err != nil {
		return nil, err
	}

	_, err = oras.Copy(ctx, r.store, ref, to, "",
		oras.WithAllowedMediaType(mt),
		oras.WithLayerDescriptors(func(descs []v1.Descriptor) {
			ldescs = append(ldescs, descs...)
		}))
	if err != nil {
		return nil, err
	}

	return ldescs, nil
}

func (r *registry) Locate(pkg v1alpha1.KeviSpecPackage) string {
	ref := path.Join(r.Hostname, DefaultRepositoryNamespace, pkg.Name)
	refn, err := name.ParseReference(ref)
	if err != nil {
		return path.Join(r.Hostname, DefaultRepositoryNamespace, pkg.Name)
	}
	return refn.Name()
}

func (r *registry) contentMediaType(pkg v1alpha1.KeviSpecPackage) (string, error) {
	switch pkg.Identify() {
	case v1alpha1.KeviPackageManifestType:
		return v1alpha1.ManifestLayerMediaType, nil
	case v1alpha1.KeviPackageChartType:
		return consts.ChartLayerMediaType, nil
	}
	return "", fmt.Errorf("unknown kevi package type")
}
