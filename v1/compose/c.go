package compose

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v2"
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
	sort.SliceStable(c.Layers, NewLayerComparator(c.Layers))

	in, err := c.fs.ReadFile(c.Base)
	if err != nil {
		return "", fmt.Errorf("failed to read base compose file: %s", err)
	}

	var b map[interface{}]interface{}
	err = yaml.Unmarshal(in, &b)
	if err != nil {
		return "", fmt.Errorf("failed to parse base compose file: %s", err)
	}

	for _, layer := range c.Layers {
		in, err := c.fs.ReadFile(fmt.Sprintf("%s.d/%s", c.Base, layer))
		if err != nil {
			return "", fmt.Errorf("failed to read layer compose file: %s", err)
		}

		var l map[interface{}]interface{}
		err = yaml.Unmarshal(in, &l)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file: %s", err)
		}

		for k, v := range l {
			b[k] = v
		}
	}

	out, err := yaml.Marshal(b)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %s", err)
	}

	return string(out), nil
}

func part(s string) (string, string) {
	p := strings.IndexRune(s, '-')
	if p == -1 {
		panic(fmt.Sprintf("invalid argument s: %s", s))
	}
	return s[:p], s[p+1:]
}
