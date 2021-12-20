package pack_test

import (
	"testing"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/pack"
)

func TestChart_Generate(t *testing.T) {

	type args struct {
		name string
		pkg  v1alpha1.KeviSpecPackageChart
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "should work with a local chart",
			args: args{
				name: "local compressed",
				pkg: v1alpha1.KeviSpecPackageChart{
					Path: "../../testdata/podinfo-6.0.3.tgz",
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			// TODO: If we were doing this right, we'd use a mock chart server
			name: "should work with a remote chart",
			args: args{
				name: "remote",
				pkg: v1alpha1.KeviSpecPackageChart{
					Name:    "loki",
					RepoUrl: "https://grafana.github.io/helm-charts",
				},
			},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := pack.NewChart(tt.args.name, tt.args.pkg)
			if err != nil {
				t.Fatal(err)
			}

			got, err := c.Generate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			_ = got

			cnts, err := c.Contents()
			if err != nil {
				t.Fatal(err)
			}
			_ = cnts
		})
	}
}
