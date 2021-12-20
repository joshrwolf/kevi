package pack

import (
	"context"
	"fmt"

	"github.com/rancherfederal/ocil/pkg/artifacts"
	"helm.sh/helm/v3/pkg/chart/loader"
	"oras.land/oras-go/pkg/content"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
)

// Package represents deployable content kevi understands that can be packaged and loaded
type Package interface {
	artifacts.OCICollection

	Generate() ([]byte, error)
}

func Load(ctx context.Context, fetcher fetcher.Fetcher, pkg v1alpha1.KeviSpecPackage) (Package, error) {
	mfs := content.NewMemory()

	descs, err := fetcher.Fetch(ctx, mfs, pkg)
	if err != nil {
		return nil, err
	}

	switch pkg.Identify() {
	case v1alpha1.KeviPackageManifestType:
		if len(descs) != 1 {
			return nil, fmt.Errorf("expected 1 layer, got %d", len(descs))
		}

		rc, err := mfs.Fetch(ctx, descs[0])
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		kfs, err := utgz(rc)
		if err != nil {
			return nil, err
		}

		return &Manifest{
			fs: kfs,
		}, nil

	case v1alpha1.KeviPackageChartType:
		if len(descs) != 1 {
			return nil, fmt.Errorf("expected a single chart layer, got %d", len(descs))
		}

		rc, err := mfs.Fetch(ctx, descs[0])
		if err != nil {
			return nil, err
		}
		defer rc.Close()

		ch, err := loader.LoadArchive(rc)
		if err != nil {
			return nil, err
		}

		return &Chart{
			chart: ch,
		}, nil

	default:
		return nil, fmt.Errorf("unknown kevi package type")
	}
}
