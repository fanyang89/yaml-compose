package compose

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

func NewLayerComparator(layers []string) func(i, j int) bool {
	return func(i, j int) bool {
		ap, aname := part(layers[i])
		bp, bname := part(layers[j])
		if ap != bp {
			api, err := strconv.Atoi(ap)
			if err != nil {
				panic(err)
			}
			bpi, err := strconv.Atoi(bp)
			if err != nil {
				panic(err)
			}
			return api < bpi
		}
		return strings.Compare(aname, bname) < 0
	}
}

type Compose struct {
	Base   string
	Layers []string
	fs     *afero.Afero
}

func New(base string, layers []string) *Compose {
	return &Compose{
		Base:   base,
		Layers: layers,
		fs:     &afero.Afero{Fs: afero.NewOsFs()},
	}
}

func NewMock(base string, layers []string) *Compose {
	return &Compose{
		Base:   base,
		Layers: layers,
		fs:     &afero.Afero{Fs: afero.NewMemMapFs()},
	}
}

func (c *Compose) GetFilesystem() *afero.Afero {
	return c.fs
}

func (c *Compose) Run() (string, error) {
	for _, layer := range c.Layers {
		if err := validateLayerName(layer); err != nil {
			return "", err
		}
	}

	sort.SliceStable(c.Layers, NewLayerComparator(c.Layers))

	in, err := c.fs.ReadFile(c.Base)
	if err != nil {
		return "", fmt.Errorf("failed to read base compose file: %s", err)
	}

	var b map[string]interface{}
	err = yaml.Unmarshal(in, &b)
	if err != nil {
		return "", fmt.Errorf("failed to parse base compose file: %s", err)
	}

	for _, layer := range c.Layers {
		in, err := c.fs.ReadFile(fmt.Sprintf("%s.d/%s", c.Base, layer))
		if err != nil {
			return "", fmt.Errorf("failed to read layer compose file: %s", err)
		}

		var l map[string]interface{}
		err = yaml.Unmarshal(in, &l)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file: %s", err)
		}

		b = mergeMaps(b, l)
	}

	out, err := yaml.Marshal(b)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %s", err)
	}

	return string(out), nil
}

func part(s string) (string, string) {
	left, right, ok := strings.Cut(s, "-")
	if !ok {
		return s, ""
	}
	return left, right
}

func validateLayerName(layer string) error {
	prefix, _, ok := strings.Cut(layer, "-")
	if !ok || prefix == "" {
		return fmt.Errorf("invalid layer file name %q: expected <order>-<name>.yaml", layer)
	}
	if _, err := strconv.Atoi(prefix); err != nil {
		return fmt.Errorf("invalid layer file name %q: expected numeric order prefix", layer)
	}
	return nil
}

func mergeMaps(base map[string]interface{}, layer map[string]interface{}) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}

	for k, v := range layer {
		existing, ok := base[k]
		if !ok {
			base[k] = v
			continue
		}
		base[k] = mergeValue(existing, v)
	}

	return base
}

func mergeValue(base interface{}, layer interface{}) interface{} {
	baseMap, baseIsMap := base.(map[string]interface{})
	layerMap, layerIsMap := layer.(map[string]interface{})
	if baseIsMap && layerIsMap {
		return mergeMaps(baseMap, layerMap)
	}

	return layer
}
