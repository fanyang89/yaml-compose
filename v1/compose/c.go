package compose

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
	Base             string
	Layers           []string
	ExtractLayerPath string
	fs               *afero.Afero
}

type mapMergeStrategy string

const (
	mapMergeDeep     mapMergeStrategy = "deep"
	mapMergeOverride mapMergeStrategy = "override"
)

type listMergeStrategy string

const (
	listMergeOverride listMergeStrategy = "override"
	listMergeAppend   listMergeStrategy = "append"
	listMergePrepend  listMergeStrategy = "prepend"
)

type mergeStrategy struct {
	Map  mapMergeStrategy
	List listMergeStrategy
}

var defaultMergeStrategy = mergeStrategy{
	Map:  mapMergeDeep,
	List: listMergeOverride,
}

type layerMetadata struct {
	Merge mergeMetadata `yaml:"merge"`
}

type mergeMetadata struct {
	Defaults mergeMetadataStrategy            `yaml:"defaults"`
	Paths    map[string]mergeMetadataStrategy `yaml:"paths"`
}

type mergeMetadataStrategy struct {
	Map  string `yaml:"map"`
	List string `yaml:"list"`
}

type layerMergeStrategy struct {
	defaults mergeStrategy
	paths    map[string]mergeStrategy
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

	var extractLayerParts []string
	if c.ExtractLayerPath != "" {
		parts, err := splitDotPath(c.ExtractLayerPath)
		if err != nil {
			return "", fmt.Errorf("invalid extract layer path %q: %w", c.ExtractLayerPath, err)
		}
		extractLayerParts = parts
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

		l, strategy, err := parseLayer(in)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file: %s", err)
		}

		if len(extractLayerParts) > 0 {
			extracted, ok := extractLayerAtPath(l, extractLayerParts)
			if !ok {
				continue
			}
			l = extracted
		}

		b = mergeMapsWithStrategy(b, l, strategy, nil)
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
	return mergeMapsWithStrategy(base, layer, layerMergeStrategy{defaults: defaultMergeStrategy}, nil)
}

func mergeMapsWithStrategy(base map[string]interface{}, layer map[string]interface{}, strategy layerMergeStrategy, path []string) map[string]interface{} {
	if base == nil {
		base = map[string]interface{}{}
	}

	for k, v := range layer {
		existing, ok := base[k]
		if !ok {
			base[k] = v
			continue
		}
		nextPath := appendPath(path, k)
		base[k] = mergeValue(existing, v, strategy, nextPath)
	}

	return base
}

func mergeValue(base interface{}, layer interface{}, strategy layerMergeStrategy, path []string) interface{} {
	baseMap, baseIsMap := base.(map[string]interface{})
	layerMap, layerIsMap := layer.(map[string]interface{})
	if baseIsMap && layerIsMap {
		pathStrategy := strategy.resolve(path)
		if pathStrategy.Map == mapMergeOverride {
			return layerMap
		}
		return mergeMapsWithStrategy(baseMap, layerMap, strategy, path)
	}

	baseList, baseIsList := base.([]interface{})
	layerList, layerIsList := layer.([]interface{})
	if baseIsList && layerIsList {
		switch strategy.resolve(path).List {
		case listMergeAppend:
			out := make([]interface{}, 0, len(baseList)+len(layerList))
			out = append(out, baseList...)
			out = append(out, layerList...)
			return out
		case listMergePrepend:
			out := make([]interface{}, 0, len(baseList)+len(layerList))
			out = append(out, layerList...)
			out = append(out, baseList...)
			return out
		default:
			return layerList
		}
	}

	return layer
}

func parseLayer(in []byte) (map[string]interface{}, layerMergeStrategy, error) {
	docs, err := decodeYAMLDocuments(in)
	if err != nil {
		return nil, layerMergeStrategy{}, err
	}

	switch len(docs) {
	case 0:
		return map[string]interface{}{}, layerMergeStrategy{defaults: defaultMergeStrategy}, nil
	case 1:
		data, err := decodeYAMLMap(docs[0])
		if err != nil {
			return nil, layerMergeStrategy{}, err
		}
		return data, layerMergeStrategy{defaults: defaultMergeStrategy}, nil
	case 2:
		meta, err := decodeLayerMetadata(docs[0])
		if err != nil {
			return nil, layerMergeStrategy{}, err
		}
		strategy, err := buildLayerMergeStrategy(meta)
		if err != nil {
			return nil, layerMergeStrategy{}, err
		}
		data, err := decodeYAMLMap(docs[1])
		if err != nil {
			return nil, layerMergeStrategy{}, err
		}
		return data, strategy, nil
	default:
		return nil, layerMergeStrategy{}, fmt.Errorf("expected at most two YAML documents (metadata and data), got %d", len(docs))
	}
}

func decodeYAMLDocuments(in []byte) ([]*yaml.Node, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(in))
	docs := make([]*yaml.Node, 0)
	for {
		var doc yaml.Node
		err := decoder.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(doc.Content) == 0 {
			continue
		}
		docs = append(docs, &doc)
	}
	return docs, nil
}

func decodeYAMLMap(doc *yaml.Node) (map[string]interface{}, error) {
	var m map[string]interface{}
	if err := doc.Decode(&m); err != nil {
		return nil, fmt.Errorf("expected YAML mapping document: %w", err)
	}
	if m == nil {
		return map[string]interface{}{}, nil
	}
	return m, nil
}

func decodeLayerMetadata(doc *yaml.Node) (layerMetadata, error) {
	var meta layerMetadata
	if err := doc.Decode(&meta); err != nil {
		return layerMetadata{}, fmt.Errorf("failed to decode metadata document: %w", err)
	}
	return meta, nil
}

func buildLayerMergeStrategy(meta layerMetadata) (layerMergeStrategy, error) {
	strategy := layerMergeStrategy{
		defaults: defaultMergeStrategy,
		paths:    map[string]mergeStrategy{},
	}

	var err error
	strategy.defaults, err = applyMetadataStrategy(strategy.defaults, meta.Merge.Defaults)
	if err != nil {
		return layerMergeStrategy{}, fmt.Errorf("invalid merge.defaults: %w", err)
	}

	for rawPath, override := range meta.Merge.Paths {
		normalizedPath, err := normalizeDotPath(rawPath)
		if err != nil {
			return layerMergeStrategy{}, fmt.Errorf("invalid merge.paths.%q: %w", rawPath, err)
		}

		pathStrategy, err := applyMetadataStrategy(strategy.defaults, override)
		if err != nil {
			return layerMergeStrategy{}, fmt.Errorf("invalid merge.paths.%q: %w", rawPath, err)
		}
		strategy.paths[normalizedPath] = pathStrategy
	}

	return strategy, nil
}

func applyMetadataStrategy(base mergeStrategy, override mergeMetadataStrategy) (mergeStrategy, error) {
	ret := base
	if override.Map != "" {
		mapStrategy, err := parseMapMergeStrategy(override.Map)
		if err != nil {
			return mergeStrategy{}, err
		}
		ret.Map = mapStrategy
	}

	if override.List != "" {
		listStrategy, err := parseListMergeStrategy(override.List)
		if err != nil {
			return mergeStrategy{}, err
		}
		ret.List = listStrategy
	}

	return ret, nil
}

func parseMapMergeStrategy(s string) (mapMergeStrategy, error) {
	switch mapMergeStrategy(s) {
	case mapMergeDeep, mapMergeOverride:
		return mapMergeStrategy(s), nil
	default:
		return "", fmt.Errorf("unsupported map strategy %q", s)
	}
}

func parseListMergeStrategy(s string) (listMergeStrategy, error) {
	switch listMergeStrategy(s) {
	case listMergeOverride, listMergeAppend, listMergePrepend:
		return listMergeStrategy(s), nil
	default:
		return "", fmt.Errorf("unsupported list strategy %q", s)
	}
}

func (s layerMergeStrategy) resolve(path []string) mergeStrategy {
	if len(s.paths) == 0 {
		return s.defaults
	}
	normalizedPath := normalizePath(path)
	if pathStrategy, ok := s.paths[normalizedPath]; ok {
		return pathStrategy
	}
	return s.defaults
}

func appendPath(path []string, key string) []string {
	next := make([]string, len(path)+1)
	copy(next, path)
	next[len(path)] = key
	return next
}

func extractLayerAtPath(layer map[string]interface{}, path []string) (map[string]interface{}, bool) {
	if len(path) == 0 {
		return layer, true
	}

	var cur interface{} = layer
	for _, segment := range path {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		next, ok := m[segment]
		if !ok {
			return nil, false
		}
		cur = next
	}

	ret := cur
	for i := len(path) - 1; i >= 0; i-- {
		ret = map[string]interface{}{path[i]: ret}
	}

	wrapped, ok := ret.(map[string]interface{})
	if !ok {
		return nil, false
	}
	return wrapped, true
}

func normalizeDotPath(path string) (string, error) {
	parts, err := splitDotPath(path)
	if err != nil {
		return "", err
	}
	return normalizePath(parts), nil
}

func splitDotPath(path string) ([]string, error) {
	if path == "" {
		return nil, fmt.Errorf("path cannot be empty")
	}

	parts := make([]string, 0)
	var segment strings.Builder
	escaped := false
	for i := 0; i < len(path); i++ {
		ch := path[i]
		if escaped {
			segment.WriteByte(ch)
			escaped = false
			continue
		}

		switch ch {
		case '\\':
			escaped = true
		case '.':
			if segment.Len() == 0 {
				return nil, fmt.Errorf("empty segment")
			}
			parts = append(parts, segment.String())
			segment.Reset()
		default:
			segment.WriteByte(ch)
		}
	}

	if escaped {
		return nil, fmt.Errorf("path cannot end with an escape character")
	}
	if segment.Len() == 0 {
		return nil, fmt.Errorf("empty segment")
	}
	parts = append(parts, segment.String())

	return parts, nil
}

func normalizePath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	encoded := make([]string, 0, len(parts))
	for _, part := range parts {
		encoded = append(encoded, escapeDotPathSegment(part))
	}
	return strings.Join(encoded, ".")
}

func escapeDotPathSegment(part string) string {
	part = strings.ReplaceAll(part, `\\`, `\\\\`)
	part = strings.ReplaceAll(part, ".", `\\.`)
	return part
}
