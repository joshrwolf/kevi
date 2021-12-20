package pack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rancherfederal/ocil/pkg/artifacts"
	"github.com/rancherfederal/ocil/pkg/artifacts/image"
	"github.com/rancherfederal/ocil/pkg/artifacts/memory"
	"github.com/rancherfederal/ocil/pkg/consts"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/kube/fake"
	"helm.sh/helm/v3/pkg/storage"
	"helm.sh/helm/v3/pkg/storage/driver"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
)

var (
	_ Package = &Chart{}
)

type Option func(*Chart)

func WithChart(ch *chart.Chart) Option {
	return func(c *Chart) {
		c.chart = ch
	}
}

type Chart struct {
	Name string

	chart *chart.Chart
	path  string
}

func NewChart(name string, cpkg v1alpha1.KeviSpecPackageChart) (*Chart, error) {
	var (
		ch   *chart.Chart
		err  error
		path string
	)

	if cpkg.Path != "" {
		path = cpkg.Path
		ch, err = loader.Load(cpkg.Path)

	} else if cpkg.RepoUrl != "" {
		cpo := action.ChartPathOptions{
			RepoURL: cpkg.RepoUrl,
			Version: cpkg.Version,
		}
		cp, err := cpo.LocateChart(cpkg.Name, cli.New())
		if err != nil {
			return nil, err
		}

		path = cp
		ch, err = loader.Load(cp)

	} else {
		return nil, fmt.Errorf("couldn't identify a valid chart location, you must specify a path or repourl")
	}
	if err != nil {
		return nil, err
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	return &Chart{
		Name:  name,
		chart: ch,
		path:  abs,
	}, nil
}

func (c *Chart) Contents() (map[string]artifacts.OCI, error) {
	coll := make(map[string]artifacts.OCI)

	data, err := c.Generate()
	if err != nil {
		return nil, err
	}

	imagesFound := find(data, defaultKnownImagePaths...)
	for _, i := range imagesFound {
		refn, err := name.ParseReference(i)
		if err != nil {
			return nil, err
		}

		img, err := image.NewImage(refn.Name())
		if err != nil {
			return nil, err
		}

		coll[refn.Name()] = img
	}

	// tar and compress the chart data
	chdata, err := c.tgz()
	if err != nil {
		return nil, err
	}

	coll[c.Reference()] = memory.NewMemory(chdata,
		consts.ChartLayerMediaType,
		memory.WithConfig(c.chart.Metadata, consts.ChartConfigMediaType),
	)
	return coll, nil
}

func (c *Chart) Reference() string {
	return path.Join(fetcher.DefaultRepositoryNamespace, c.Name)
}

func (c *Chart) Generate() ([]byte, error) {
	s := storage.Init(driver.NewMemory())
	cfg := &action.Configuration{
		Releases:     s,
		KubeClient:   &fake.PrintingKubeClient{Out: io.Discard},
		Capabilities: chartutil.DefaultCapabilities,
		Log:          func(s string, i ...interface{}) {},
	}
	client := action.NewInstall(cfg)
	client.ReleaseName = "dry"
	client.DryRun = true
	client.Replace = true
	client.ClientOnly = true
	client.IncludeCRDs = true

	vals := make(map[string]interface{})
	release, err := client.Run(c.chart, vals)
	if err != nil {
		return nil, err
	}

	return []byte(release.Manifest), nil
}

// tgz returns the data of a gzip compressed archive of the chart
// 	Because this only ever runs with access to a real filesystem, we can simply read/create .tgz's directly from the filesystem
func (c *Chart) tgz() ([]byte, error) {
	if c.path == "" {
		return nil, fmt.Errorf("can't compute without a path")
	}
	ext := filepath.Ext(c.path)
	if ext == ".tgz" || ext == ".tar.gz" {
		// don't build one if we don't need to
		return os.ReadFile(c.path)
	}

	// Asume we have a chart directory, compress it!
	info, err := os.Stat(c.path)
	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("expected chart directory but got file")
	}

	var b bytes.Buffer
	gw := gzip.NewWriter(&b)
	tw := tar.NewWriter(gw)
	if err := filepath.WalkDir(c.path, func(path string, d fs.DirEntry, err error) error {
		fi, err := d.Info()
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(filepath.Dir(c.path), path)
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
	return b.Bytes(), nil
}
