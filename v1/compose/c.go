package compose

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type marshalFunc func(interface{}) ([]byte, error)

const yamlOutputIndentSpaces = 2

func NewLayerComparator(layers []string) func(i, j int) bool {
	return func(i, j int) bool {
		ap, aname := part(layers[i])
		bp, bname := part(layers[j])

		api, aerr := strconv.Atoi(ap)
		bpi, berr := strconv.Atoi(bp)
		if aerr == nil && berr == nil {
			if api != bpi {
				return api < bpi
			}
			return strings.Compare(aname, bname) < 0
		}

		if aerr == nil {
			return true
		}
		if berr == nil {
			return false
		}

		if ap != bp {
			return strings.Compare(ap, bp) < 0
		}
		return strings.Compare(aname, bname) < 0
	}
}

type Compose struct {
	Base    string
	Layers  []string
	fs      *afero.Afero
	marshal marshalFunc
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
	Merge     mergeMetadata           `yaml:"merge"`
	Transform *layerTransformMetadata `yaml:"transform"`
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

type layerTransformMetadata struct {
	Kind       string                  `yaml:"kind"`
	Source     layerTransformSource    `yaml:"source"`
	Target     layerTransformTarget    `yaml:"target"`
	ListFilter layerListFilterMetadata `yaml:"list_filter"`
}

type layerTransformSource struct {
	File string `yaml:"file"`
	Path string `yaml:"path"`
}

type layerTransformTarget struct {
	Path string `yaml:"path"`
}

type layerListFilterMetadata struct {
	MatchPath   string   `yaml:"match_path"`
	Include     []string `yaml:"include"`
	Exclude     []string `yaml:"exclude"`
	IncludeMode string   `yaml:"include_mode"`
}

type layerTransform struct {
	sourceFile string
	sourcePath []string
	targetPath []string
	listFilter layerListFilter
}

type layerListFilter struct {
	matchPath   []string
	include     []*regexp.Regexp
	exclude     []*regexp.Regexp
	includeMode includeMode
}

type includeMode string

const (
	includeModeAny includeMode = "any"
	includeModeAll includeMode = "all"
)

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
	}
}

func marshalYAML(in interface{}) ([]byte, error) {
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

		l, strategy, transform, err := parseLayer(in)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file: %s", err)
		}

		if transform != nil {
			l, err = c.applyLayerTransform(l, transform)
			if err != nil {
				return "", fmt.Errorf("failed to apply layer transform for %q: %w", layer, err)
			}
		}

		b = mergeMapsWithStrategy(b, l, strategy, nil)
	}

	out, err := c.marshal(b)
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

func parseLayer(in []byte) (map[string]interface{}, layerMergeStrategy, *layerTransform, error) {
	docs, err := decodeYAMLDocuments(in)
	if err != nil {
		return nil, layerMergeStrategy{}, nil, err
	}

	switch len(docs) {
	case 0:
		return map[string]interface{}{}, layerMergeStrategy{defaults: defaultMergeStrategy}, nil, nil
	case 1:
		data, err := decodeYAMLMap(docs[0])
		if err != nil {
			return nil, layerMergeStrategy{}, nil, err
		}
		return data, layerMergeStrategy{defaults: defaultMergeStrategy}, nil, nil
	case 2:
		meta, err := decodeLayerMetadata(docs[0])
		if err != nil {
			return nil, layerMergeStrategy{}, nil, err
		}
		strategy, err := buildLayerMergeStrategy(meta)
		if err != nil {
			return nil, layerMergeStrategy{}, nil, err
		}
		transform, err := buildLayerTransform(meta)
		if err != nil {
			return nil, layerMergeStrategy{}, nil, err
		}
		data, err := decodeYAMLMap(docs[1])
		if err != nil {
			return nil, layerMergeStrategy{}, nil, err
		}
		return data, strategy, transform, nil
	default:
		return nil, layerMergeStrategy{}, nil, fmt.Errorf("expected at most two YAML documents (metadata and data), got %d", len(docs))
	}
}

func buildLayerTransform(meta layerMetadata) (*layerTransform, error) {
	if meta.Transform == nil {
		return nil, nil
	}

	if meta.Transform.Kind != "list_filter" {
		return nil, fmt.Errorf("invalid transform.kind %q: supported values: list_filter", meta.Transform.Kind)
	}

	if meta.Transform.Source.File == "" {
		return nil, fmt.Errorf("invalid transform.source.file: cannot be empty")
	}

	sourcePath, err := splitDotPath(meta.Transform.Source.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid transform.source.path %q: %w", meta.Transform.Source.Path, err)
	}

	targetPathRaw := meta.Transform.Target.Path
	if targetPathRaw == "" {
		targetPathRaw = meta.Transform.Source.Path
	}
	targetPath, err := splitDotPath(targetPathRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid transform.target.path %q: %w", targetPathRaw, err)
	}

	filter, err := buildListFilter(meta.Transform.ListFilter)
	if err != nil {
		return nil, err
	}

	return &layerTransform{
		sourceFile: meta.Transform.Source.File,
		sourcePath: sourcePath,
		targetPath: targetPath,
		listFilter: filter,
	}, nil
}

func buildListFilter(meta layerListFilterMetadata) (layerListFilter, error) {
	includeRegex, err := compileRegexList(meta.Include, "transform.list_filter.include")
	if err != nil {
		return layerListFilter{}, err
	}

	excludeRegex, err := compileRegexList(meta.Exclude, "transform.list_filter.exclude")
	if err != nil {
		return layerListFilter{}, err
	}

	mode := includeModeAny
	if meta.IncludeMode != "" {
		mode = includeMode(meta.IncludeMode)
	}
	if mode != includeModeAny && mode != includeModeAll {
		return layerListFilter{}, fmt.Errorf("invalid transform.list_filter.include_mode %q: supported values: any, all", meta.IncludeMode)
	}

	var matchPath []string
	if meta.MatchPath != "" {
		matchPath, err = splitDotPath(meta.MatchPath)
		if err != nil {
			return layerListFilter{}, fmt.Errorf("invalid transform.list_filter.match_path %q: %w", meta.MatchPath, err)
		}
	}

	return layerListFilter{
		matchPath:   matchPath,
		include:     includeRegex,
		exclude:     excludeRegex,
		includeMode: mode,
	}, nil
}

func compileRegexList(raw []string, fieldName string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(raw))
	for i, expr := range raw {
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s[%d] %q: %w", fieldName, i, expr, err)
		}
		out = append(out, re)
	}
	return out, nil
}

func (c *Compose) applyLayerTransform(layer map[string]interface{}, transform *layerTransform) (map[string]interface{}, error) {
	if layer == nil {
		layer = map[string]interface{}{}
	}

	sourceData, err := c.readSourceYAML(transform.sourceFile)
	if err != nil {
		return nil, err
	}

	input, ok := getValueAtPath(sourceData, transform.sourcePath)
	if !ok {
		return nil, fmt.Errorf("source path %q not found", normalizePath(transform.sourcePath))
	}

	inputList, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("source path %q must resolve to a list", normalizePath(transform.sourcePath))
	}

	outputList, err := applyListFilter(inputList, transform.listFilter)
	if err != nil {
		return nil, err
	}

	if err := setMapValueAtPath(layer, transform.targetPath, outputList); err != nil {
		return nil, err
	}

	return layer, nil
}

func (c *Compose) readSourceYAML(rawPath string) (interface{}, error) {
	resolvedPath := rawPath
	if !filepath.IsAbs(rawPath) {
		resolvedPath = filepath.Clean(filepath.Join(filepath.Dir(c.Base), rawPath))
	}

	in, err := c.fs.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read transform source file %q: %w", resolvedPath, err)
	}

	var out interface{}
	if err := yaml.Unmarshal(in, &out); err != nil {
		return nil, fmt.Errorf("failed to parse transform source file %q: %w", resolvedPath, err)
	}

	return out, nil
}

func applyListFilter(input []interface{}, filter layerListFilter) ([]interface{}, error) {
	out := make([]interface{}, 0, len(input))
	for i, item := range input {
		candidate, err := getFilterCandidate(item, filter.matchPath)
		if err != nil {
			return nil, fmt.Errorf("invalid list item at index %d: %w", i, err)
		}

		if !matchesInclude(candidate, filter.include, filter.includeMode) {
			continue
		}
		if matchesAny(candidate, filter.exclude) {
			continue
		}
		out = append(out, item)
	}

	return out, nil
}

func getFilterCandidate(item interface{}, matchPath []string) (string, error) {
	if len(matchPath) == 0 {
		if _, isObject := item.(map[string]interface{}); isObject {
			return "", fmt.Errorf("object list requires transform.list_filter.match_path")
		}
		s, ok := item.(string)
		if !ok {
			return "", fmt.Errorf("expected string list item")
		}
		return s, nil
	}

	m, ok := item.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("expected object list item for match_path %q", normalizePath(matchPath))
	}

	v, ok := getValueAtPath(m, matchPath)
	if !ok {
		return "", fmt.Errorf("match_path %q not found", normalizePath(matchPath))
	}

	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("match_path %q must resolve to string", normalizePath(matchPath))
	}

	return s, nil
}

func matchesInclude(candidate string, include []*regexp.Regexp, mode includeMode) bool {
	if len(include) == 0 {
		return true
	}

	if mode == includeModeAll {
		for _, re := range include {
			if !re.MatchString(candidate) {
				return false
			}
		}
		return true
	}

	return matchesAny(candidate, include)
}

func matchesAny(candidate string, regex []*regexp.Regexp) bool {
	for _, re := range regex {
		if re.MatchString(candidate) {
			return true
		}
	}
	return false
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

func getValueAtPath(root interface{}, path []string) (interface{}, bool) {
	if len(path) == 0 {
		return root, true
	}

	cur := root
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

	return cur, true
}

func setMapValueAtPath(root map[string]interface{}, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	cur := root
	for i := 0; i < len(path)-1; i++ {
		segment := path[i]
		next, ok := cur[segment]
		if !ok {
			child := map[string]interface{}{}
			cur[segment] = child
			cur = child
			continue
		}

		nextMap, ok := next.(map[string]interface{})
		if !ok {
			return fmt.Errorf("target path %q is not writable: segment %q is not an object", normalizePath(path), normalizePath(path[:i+1]))
		}
		cur = nextMap
	}

	cur[path[len(path)-1]] = value
	return nil
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
