package pack

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/util/jsonpath"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

var defaultKnownImagePaths = []string{
	// Deployments & DaemonSets
	"{.spec.template.spec.initContainers[*].image}",
	"{.spec.template.spec.containers[*].image}",

	// Pods
	"{.spec.initContainers[*].image}",
	"{.spec.containers[*].image}",
}

func find(data []byte, paths ...string) []string {
	var (
		pathMatches []string
		obj         interface{}
	)

	reader := yaml.NewYAMLReader(bufio.NewReader(bytes.NewBuffer(data)))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		if err := yaml.Unmarshal(raw, &obj); err != nil {
			continue
		}
		j := jsonpath.New("")
		j.AllowMissingKeys(true)

		for _, p := range paths {
			r, err := parseJSONPath(obj, j, p)
			if err != nil {
				continue
			}

			pathMatches = append(pathMatches, r...)
		}
	}

	return pathMatches
}

func parseJSONPath(data interface{}, parser *jsonpath.JSONPath, template string) ([]string, error) {
	buf := new(bytes.Buffer)
	if err := parser.Parse(template); err != nil {
		return nil, err
	}

	if err := parser.Execute(buf, data); err != nil {
		return nil, err
	}

	f := func(s rune) bool { return s == ' ' }
	r := strings.FieldsFunc(buf.String(), f)
	return r, nil
}

func utgz(rc io.Reader) (filesys.FileSystem, error) {
	mfs := filesys.MakeFsInMemory()
	gr, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		switch {
		case err == io.EOF:
			return mfs, nil

		case err != nil:
			return nil, err

		case header == nil:
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := mfs.MkdirAll(header.Name); err != nil {
				return nil, err
			}

		case tar.TypeReg:
			f, err := mfs.Create(header.Name)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(f, tr); err != nil {
				return nil, err
			}
			if err := f.Close(); err != nil {
				return nil, err
			}
		}
	}
}
