package pack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rancherfederal/ocil/pkg/artifacts"
	"github.com/rancherfederal/ocil/pkg/artifacts/image"
	"github.com/rancherfederal/ocil/pkg/artifacts/memory"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/pkg/fetcher"
)

var (
	_           Package = &Manifest{}
	kbuildMutex sync.Mutex
)

type Manifest struct {
	Name string

	fs filesys.FileSystem
}

func NewManifest(name string, mpkg v1alpha1.KeviSpecPackageManifest) (*Manifest, error) {
	fsys, err := generateFS(mpkg.Path)
	if err != nil {
		return nil, err
	}

	return &Manifest{
		Name: name,

		fs: fsys,
	}, nil
}

func (m *Manifest) Reference() string {
	return path.Join(fetcher.DefaultRepositoryNamespace, m.Name)
}

func (m *Manifest) Contents() (map[string]artifacts.OCI, error) {
	coll := make(map[string]artifacts.OCI)

	data, err := m.Generate()
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

	// Build a compressed tarstream of the filesystem
	mdata, err := m.tgz()
	if err != nil {
		return nil, err
	}

	coll[m.Reference()] = memory.NewMemory(mdata, v1alpha1.ManifestLayerMediaType)
	return coll, nil
}

func (m *Manifest) Generate() ([]byte, error) {
	if err := generateRootKustomizationIfDNE(m.fs); err != nil {
		return nil, err
	}

	kbuildMutex.Lock()
	defer kbuildMutex.Unlock()

	kz, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(m.fs, ".")
	if err != nil {
		return nil, err
	}

	return kz.AsYaml()
}

func (m *Manifest) tgz() ([]byte, error) {
	var b bytes.Buffer
	gw := gzip.NewWriter(&b)
	tw := tar.NewWriter(gw)

	if err := m.fs.Walk(".", func(path string, fi fs.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		if fi.IsDir() {
			if err := m.fs.MkdirAll(path); err != nil {
				return err
			}
			return nil
		}

		rel, err := filepath.Rel("/", path)
		if err != nil {
			return err
		}
		header.Name = rel

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.IsDir() {
			src, err := m.fs.Open(rel)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, src); err != nil {
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

// generateFS will generate an in memory FS given a path of manifests
func generateFS(root string) (filesys.FileSystem, error) {
	fsys := filesys.MakeFsInMemory()
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		p, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			if p == "." {
				return nil
			}
			if err := fsys.MkdirAll(p); err != nil {
				return err
			}
			return nil
		}

		// TODO: Filter invalid manifest files
		// ext := filepath.Ext(fi.Name())
		// if ext == ".yaml" || ext == ".yml" {
		//
		// }

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := fsys.WriteFile(p, data); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return fsys, nil
}

func generateRootKustomizationIfDNE(fsys filesys.FileSystem) error {
	for _, f := range konfig.RecognizedKustomizationFileNames() {
		if _, err := fsys.ReadFile(f); err == nil {
			return nil
		}
	}

	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	var resources []string
	if err := fsys.Walk(".", func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		if ext := filepath.Ext(info.Name()); ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Ensure we have some valid yaml
		data, err := fsys.ReadFile(path)
		if err != nil {
			return err
		}
		if _, err := rf.SliceFromBytes(data); err != nil {
			return err
		}

		resources = append(resources, path)
		return nil

	}); err != nil {
		return err
	}

	kmz := &types.Kustomization{
		TypeMeta: types.TypeMeta{
			APIVersion: types.KustomizationVersion,
			Kind:       types.KustomizationKind,
		},
		Resources: resources,
	}

	data, err := yaml.Marshal(kmz)
	if err != nil {
		return err
	}

	if err := fsys.WriteFile("kustomization.yaml", data); err != nil {
		return err
	}

	return nil
}
