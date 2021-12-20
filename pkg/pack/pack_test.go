package pack

import (
	"context"
	"testing"

	packagesv1alpha1 "cattle.io/kevi/api/v1alpha1"
)

var (
	ctx context.Context
)

func TestNewOci(t *testing.T) {
	ctx = context.Background()

	pkgs := []packagesv1alpha1.KeviSpecPackage{
		{
			Name: "kustomize",
			Manifest: packagesv1alpha1.KeviSpecPackageManifest{
				Path: "../../testdata/kustomize",
			},
		},
		{
			Name: "raw",
			Manifest: packagesv1alpha1.KeviSpecPackageManifest{
				Path: "../../testdata/raw-manifests",
			},
		},
		{
			Name: "helm",
			Chart: packagesv1alpha1.KeviSpecPackageChart{
				Path: "../../testdata/podinfo-6.0.3.tgz",
			},
		},
	}

	type args struct {
		root string
	}
	tests := []struct {
		name    string
		args    args
		want    *Oci
		wantErr bool
	}{
		{
			name: "should",
			args: args{
				root: "store",
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := NewOci(tt.args.root)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewOci() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("NewOci() got = %v, want %v", got, tt.want)
			// }

			for _, pkg := range pkgs {
				desc, err := o.pack(ctx, pkg)
				if err != nil {
					t.Fatal(err)
				}
				_ = desc
			}
		})
	}
}
