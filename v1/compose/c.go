package compose

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"text/template"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type Compose struct {
	Base     string
	Layers   []string
	LayerDir string
	fs       *afero.Afero
	marshal  marshalFunc
	logOut   io.Writer
	tplVars  map[string]string
}

func New(base string, layers []string) *Compose {
	return NewWithFs(base, layers, afero.NewOsFs())
}

func NewMock(base string, layers []string) *Compose {
	return NewWithFs(base, layers, afero.NewMemMapFs())
}

func NewWithFs(base string, layers []string, fs afero.Fs) *Compose {
	return &Compose{
		Base:    base,
		Layers:  layers,
		fs:      &afero.Afero{Fs: fs},
		marshal: marshalYAML,
		logOut:  io.Discard,
	}
}

func marshalYAML(in any) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(yamlOutputIndentSpaces)
	if err := encoder.Encode(in); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (c *Compose) GetFilesystem() *afero.Afero {
	return c.fs
}

func (c *Compose) SetTransformLogWriter(w io.Writer) {
	if w == nil {
		c.logOut = io.Discard
		return
	}
	c.logOut = w
}

func (c *Compose) SetTemplateVars(vars map[string]string) {
	if len(vars) == 0 {
		c.tplVars = nil
		return
	}

	cloned := make(map[string]string, len(vars))
	for k, v := range vars {
		cloned[k] = v
	}
	c.tplVars = cloned
}

func (c *Compose) SetLayerDir(layerDir string) {
	c.LayerDir = layerDir
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

	var b map[string]any
	err = yaml.Unmarshal(in, &b)
	if err != nil {
		return "", fmt.Errorf("failed to parse base compose file: %s", err)
	}

	layerDir := c.LayerDir
	if layerDir == "" {
		layerDir = c.Base + ".d"
	}

	for i, layer := range c.Layers {
		layerPath := filepath.Join(layerDir, layer)
		in, err := c.fs.ReadFile(layerPath)
		if err != nil {
			return "", fmt.Errorf("failed to read layer compose file %q: %s", layerPath, err)
		}

		in, err = c.renderLayerTemplate(in, layer)
		if err != nil {
			return "", err
		}

		l, operators, err := parseLayer(in)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file %q: %s", layerPath, err)
		}

		for opIndex, operator := range operators {
			l, b, err = c.applyLayerOperator(l, operator, b)
			if err != nil {
				return "", fmt.Errorf("failed to apply layer operator operators[%d] (kind=%q) in layer[%d] %q: %w", opIndex, operator.kind, i, layer, err)
			}
		}
	}

	out, err := c.marshal(b)
	if err != nil {
		return "", fmt.Errorf("failed to marshal compose file: %s", err)
	}

	return string(out), nil
}

func (c *Compose) readSourceYAML(rawPath string) (any, error) {
	resolvedPath := rawPath
	if !filepath.IsAbs(rawPath) {
		resolvedPath = filepath.Clean(filepath.Join(filepath.Dir(c.Base), rawPath))
	}

	in, err := c.fs.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read transform source file %q: %w", resolvedPath, err)
	}

	var out any
	if err := yaml.Unmarshal(in, &out); err != nil {
		return nil, fmt.Errorf("failed to parse transform source file %q: %w", resolvedPath, err)
	}

	return out, nil
}

func (c *Compose) renderLayerTemplate(raw []byte, layer string) ([]byte, error) {
	if len(c.tplVars) == 0 {
		return raw, nil
	}

	tpl, err := template.New(layer).Option("missingkey=error").Parse(string(raw))
	if err != nil {
		return nil, fmt.Errorf("failed to parse layer template for %q: %w", layer, err)
	}

	var out bytes.Buffer
	if err := tpl.Execute(&out, c.tplVars); err != nil {
		return nil, fmt.Errorf("failed to render layer template for %q: %w", layer, err)
	}

	return out.Bytes(), nil
}
