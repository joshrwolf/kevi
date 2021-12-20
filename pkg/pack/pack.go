package pack

import (
	"context"
	"fmt"
	"path"

	"github.com/google/go-containerregistry/pkg/name"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/rancherfederal/ocil/pkg/artifacts"
	"github.com/rancherfederal/ocil/pkg/artifacts/memory"
	"github.com/rancherfederal/ocil/pkg/consts"
	"github.com/rancherfederal/ocil/pkg/store"
	"k8s.io/apimachinery/pkg/util/json"
	"oras.land/oras-go/pkg/content"
	"oras.land/oras-go/pkg/oras"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
)

// Packer represents a thing that knows how to package and save v1alpha1.KeviSpecPackage
type Packer interface {
	Pack(ctx context.Context, k v1alpha1.Kevi) ([]ocispec.Descriptor, error)
}

var _ Packer = &Oci{}

// oci packer
type Oci struct {
	*store.OCI
}

func (o *Oci) CopyAll(ctx context.Context, registry string, opts content.RegistryOptions) ([]ocispec.Descriptor, error) {
	r, err := content.NewRegistry(opts)
	if err != nil {
		return nil, err
	}

	var descs []ocispec.Descriptor
	if walkErr := o.Walk(func(reference string, desc ocispec.Descriptor) error {
		toRef, err := Relocate(reference, registry)
		if err != nil {
			return err
		}

		pushedDesc, err := oras.Copy(ctx, o.OCI, reference, r, toRef,
			oras.WithAdditionalCachedMediaTypes(consts.DockerManifestSchema2))
		descs = append(descs, pushedDesc)
		return err
	}); walkErr != nil {
		return nil, walkErr
	}
	return descs, err
}

func (o *Oci) Pack(ctx context.Context, k v1alpha1.Kevi) ([]ocispec.Descriptor, error) {
	var descs []ocispec.Descriptor
	for _, pkg := range k.Spec.Packages {
		ds, err := o.pack(ctx, pkg)
		if err != nil {
			return nil, err
		}
		descs = append(descs, ds...)
	}

	pkgData, err := json.Marshal(k)
	if err != nil {
		return nil, err
	}
	pm := memory.NewMemory(pkgData, v1alpha1.KeviPackageLayerMediaType)

	pkgref := path.Join(fetcher.DefaultRepositoryNamespace, "kevi-"+k.Name)
	pkgDesc, err := o.AddOCI(ctx, pm, pkgref)
	if err != nil {
		return nil, err
	}

	descs = append(descs, pkgDesc)
	return descs, nil
}

func (o *Oci) pack(ctx context.Context, pkg v1alpha1.KeviSpecPackage) ([]ocispec.Descriptor, error) {
	var coll artifacts.OCICollection

	switch pkg.Identify() {
	case v1alpha1.KeviPackageManifestType:
		m, err := NewManifest(pkg.Name, pkg.Manifest)
		if err != nil {
			return nil, err
		}
		coll = m

	case v1alpha1.KeviPackageChartType:
		c, err := NewChart(pkg.Name, pkg.Chart)
		if err != nil {
			return nil, err
		}
		coll = c

	default:
		return nil, fmt.Errorf("unknown kevi package type")
	}

	return o.AddOCICollection(ctx, coll)
}

func NewOci(root string) (*Oci, error) {
	soci, err := store.NewOCI(root)
	if err != nil {
		return nil, err
	}

	return &Oci{
		OCI: soci,
	}, nil
}

func Relocate(ref string, registry string) (string, error) {
	or, err := name.ParseReference(ref)
	if err != nil {
		return "", err
	}

	relocated, err := name.ParseReference(or.Context().RepositoryStr(), name.WithDefaultRegistry(registry))
	if err != nil {
		return "", err
	}

	if _, err := name.NewDigest(or.Name()); err == nil {
		return relocated.Context().Digest(or.Identifier()).Name(), nil
	}
	return relocated.Context().Tag(or.Identifier()).Name(), nil

}
