package builder

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	pathPkg "path"

	yaml "gopkg.in/yaml.v2"
)

type MetadataPartsDirectoryReader struct {
	filesystem  filesystem
	topLevelKey string
	orderKey    string
}

type Part struct {
	File     string
	Name     string
	Metadata interface{}
}

func NewMetadataPartsDirectoryReader(filesystem filesystem, topLevelKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{filesystem: filesystem, topLevelKey: topLevelKey}
}

func NewMetadataPartsDirectoryReaderWithOrder(filesystem filesystem, topLevelKey, orderKey string) MetadataPartsDirectoryReader {
	return MetadataPartsDirectoryReader{filesystem: filesystem, topLevelKey: topLevelKey, orderKey: orderKey}
}

func (r MetadataPartsDirectoryReader) Read(path string) ([]Part, error) {
	parts, err := r.readMetadataRecursivelyFromDir(path)
	if err != nil {
		return []Part{}, err
	}

	if r.orderKey != "" {
		return r.orderWithOrderFile(path, parts)
	}

	return r.orderAlphabeticallyByName(path, parts)
}

func (r MetadataPartsDirectoryReader) readMetadataRecursivelyFromDir(path string) ([]Part, error) {
	parts := []Part{}

	err := r.filesystem.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(filePath) != ".yml" || pathPkg.Base(filePath) == "_order.yml" {
			return nil
		}

		f, err := r.filesystem.Open(filePath)
		if err != nil {
			return err
		}
		defer f.Close()

		data, err := ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		var fileVars map[string]interface{}
		err = yaml.Unmarshal([]byte(data), &fileVars)
		if err != nil {
			return err
		}

		vars, ok := fileVars[r.topLevelKey]
		if !ok {
			return fmt.Errorf("not a %s file: %q", r.topLevelKey, filePath)
		}

		parts, err = r.readMetadataIntoParts(pathPkg.Base(filePath), vars, parts)
		if err != nil {
			return fmt.Errorf("file '%s' with top-level key '%s' has an invalid format: %s", filePath, r.topLevelKey, err)
		}

		return nil
	})

	return parts, err
}

func (r MetadataPartsDirectoryReader) readMetadataIntoParts(fileName string, vars interface{}, parts []Part) ([]Part, error) {
	switch v := vars.(type) {
	case []interface{}:
		for _, item := range v {
			i, ok := item.(map[interface{}]interface{})
			if !ok {
				return []Part{}, fmt.Errorf("metadata item '%v' must be a map", item)
			}
			name, ok := i["name"].(string)
			if !ok {
				return []Part{}, fmt.Errorf("metadata item '%v' does not have a `name` field", item)
			}
			parts = append(parts, Part{File: fileName, Name: name, Metadata: item})
		}
	case map[interface{}]interface{}:
		name, ok := v["name"].(string)
		if !ok {
			return []Part{}, fmt.Errorf("metadata item '%v' does not have a `name` field", v)
		}
		parts = append(parts, Part{File: fileName, Name: name, Metadata: v})
	default:
		return []Part{}, fmt.Errorf("expected either slice or map value")
	}

	return parts, nil
}

func (r MetadataPartsDirectoryReader) orderWithOrderFile(path string, parts []Part) ([]Part, error) {

	orderPath := filepath.Join(path, "_order.yml")
	f, err := r.filesystem.Open(orderPath)
	if err != nil {
		return []Part{}, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return []Part{}, err
	}

	var files map[string][]interface{}
	err = yaml.Unmarshal([]byte(data), &files)
	if err != nil {
		return []Part{}, fmt.Errorf("Invalid format for '%s': %s", orderPath, err)
	}

	orderedNames, ok := files[r.orderKey]
	if !ok {
		return []Part{}, fmt.Errorf("Could not find top-level order key '%s' in '%s'", r.orderKey, orderPath)
	}

	var outputs []Part
	for _, name := range orderedNames {
		found := false
		for _, part := range parts {
			if part.Name == name {
				found = true
				outputs = append(outputs, part)
			}
		}
		if !found {
			return []Part{}, fmt.Errorf("file specified in _order.yml %q does not exist in %q", name, path)
		}
	}

	return outputs, err
}

func (r MetadataPartsDirectoryReader) orderAlphabeticallyByName(path string, parts []Part) ([]Part, error) {
	var orderedKeys []string
	for _, part := range parts {
		orderedKeys = append(orderedKeys, part.Name)
	}
	sort.Strings(orderedKeys)

	var outputs []Part
	for _, name := range orderedKeys {
		for _, part := range parts {
			if part.Name == name {
				outputs = append(outputs, part)
			}
		}
	}

	return outputs, nil
}
