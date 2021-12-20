package fetcher

import (
	"context"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/pkg/target"

	"cattle.io/kevi/api/v1alpha1"
)

type Fetcher interface {
	Fetch(ctx context.Context, to target.Target, pkg v1alpha1.KeviSpecPackage) ([]v1.Descriptor, error)

	Locate(pkg v1alpha1.KeviSpecPackage) string
}
