package pack_test

import (
	"context"
	"testing"

	"oras.land/oras-go/pkg/content"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
	"cattle.io/kevi/pkg/pack"
)

func TestLoad(t *testing.T) {
	ctx := context.Background()
	f, err := fetcher.NewRegistry("registry.kabbages.co", content.RegistryOptions{})
	if err != nil {
		t.Fatal(err)
	}

	type args struct {
		ctx     context.Context
		fetcher fetcher.Fetcher
		pkg     v1alpha1.KeviSpecPackage
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "raw-manifests",
			args: args{
				ctx:     ctx,
				fetcher: f,
				pkg: v1alpha1.KeviSpecPackage{
					Name: "raw-manifests",
					Manifest: v1alpha1.KeviSpecPackageManifest{
						Path: "testdata/raw-manifests/",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "chart",
			args: args{
				ctx:     ctx,
				fetcher: f,
				pkg: v1alpha1.KeviSpecPackage{
					Name: "podinfo",
					Chart: v1alpha1.KeviSpecPackageChart{
						Path: "testdata/podinfo-6.0.3.tgz",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pack.Load(tt.args.ctx, tt.args.fetcher, tt.args.pkg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			data, err := got.Generate()
			if err != nil {
				t.Fatal(err)
				return
			}
			_ = data
		})
	}
}
