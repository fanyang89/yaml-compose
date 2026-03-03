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
	logOut  io.Writer
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
	Operators []layerOperatorMetadata `yaml:"operators"`
}

type layerOperatorMetadata struct {
	Kind        string                     `yaml:"kind"`
	Source      layerTransformSource       `yaml:"source"`
	Target      layerTransformTarget       `yaml:"target"`
	Merge       mergeMetadata              `yaml:"merge"`
	ListFilter  layerListFilterMetadata    `yaml:"list_filter"`
	ListExtract layerListExtractMetadata   `yaml:"list_extract"`
	ReplaceVals layerReplaceValuesMetadata `yaml:"replace_values"`
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
	Kind        string                     `yaml:"kind"`
	Source      layerTransformSource       `yaml:"source"`
	Target      layerTransformTarget       `yaml:"target"`
	ListFilter  layerListFilterMetadata    `yaml:"list_filter"`
	ListExtract layerListExtractMetadata   `yaml:"list_extract"`
	ReplaceVals layerReplaceValuesMetadata `yaml:"replace_values"`
}

type layerTransformSource struct {
	From string `yaml:"from"`
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

type layerListExtractMetadata struct {
	ExtractPath string   `yaml:"extract_path"`
	Include     []string `yaml:"include"`
	Exclude     []string `yaml:"exclude"`
	IncludeMode string   `yaml:"include_mode"`
}

type layerReplaceValuesMetadata struct {
	Old           string `yaml:"old"`
	New           string `yaml:"new"`
	Recursive     bool   `yaml:"recursive"`
	PrintOriginal bool   `yaml:"print_original"`
}

type layerTransform struct {
	kind          string
	sourceFrom    string
	sourceFile    string
	sourcePath    []string
	hasSourcePath bool
	targetPath    []string
	listFilter    layerListFilter
	listExtract   layerListExtract
	replaceVals   layerReplaceValues
	merge         layerMergeStrategy
}

type parsedOperatorSource struct {
	from    string
	file    string
	path    []string
	hasPath bool
}

type operatorExecutionResult struct {
	state       map[string]interface{}
	output      interface{}
	writeTarget bool
}

type layerListFilter struct {
	matchPath   []string
	include     []*regexp.Regexp
	exclude     []*regexp.Regexp
	includeMode includeMode
}

type layerListExtract struct {
	extractPath []string
	include     []*regexp.Regexp
	exclude     []*regexp.Regexp
	includeMode includeMode
}

type layerReplaceValues struct {
	old           string
	new           string
	recursive     bool
	printOriginal bool
}

type regexFilterConfig struct {
	include     []*regexp.Regexp
	exclude     []*regexp.Regexp
	includeMode includeMode
}

type includeMode string

const (
	transformKindMerge       = "merge"
	transformKindListFilter  = "list_filter"
	transformKindListExtract = "list_extract"
	transformKindReplaceVals = "replace_values"

	transformSourceFile  = "file"
	transformSourceState = "state"
	transformSourceLayer = "layer"

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
		logOut:  io.Discard,
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

func (c *Compose) SetTransformLogWriter(w io.Writer) {
	if w == nil {
		c.logOut = io.Discard
		return
	}
	c.logOut = w
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

		l, operators, err := parseLayer(in)
		if err != nil {
			return "", fmt.Errorf("failed to parse layer compose file: %s", err)
		}

		for _, operator := range operators {
			l, b, err = c.applyLayerOperator(l, operator, b)
			if err != nil {
				return "", fmt.Errorf("failed to apply layer operator for %q: %w", layer, err)
			}
		}
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

func parseLayer(in []byte) (map[string]interface{}, []layerTransform, error) {
	docs, err := decodeYAMLDocuments(in)
	if err != nil {
		return nil, nil, err
	}

	switch len(docs) {
	case 0:
		return map[string]interface{}{}, []layerTransform{defaultMergeOperator()}, nil
	case 1:
		data, err := decodeYAMLMap(docs[0])
		if err != nil {
			return nil, nil, err
		}
		return data, []layerTransform{defaultMergeOperator()}, nil
	case 2:
		meta, err := decodeLayerMetadata(docs[0])
		if err != nil {
			return nil, nil, err
		}
		operators, err := buildLayerOperators(meta)
		if err != nil {
			return nil, nil, err
		}
		data, err := decodeYAMLMap(docs[1])
		if err != nil {
			return nil, nil, err
		}
		return data, operators, nil
	default:
		return nil, nil, fmt.Errorf("expected at most two YAML documents (metadata and data), got %d", len(docs))
	}
}

func defaultMergeOperator() layerTransform {
	return layerTransform{
		kind:       transformKindMerge,
		sourceFrom: transformSourceLayer,
		merge: layerMergeStrategy{
			defaults: defaultMergeStrategy,
			paths:    map[string]mergeStrategy{},
		},
	}
}

func buildLayerOperators(meta layerMetadata) ([]layerTransform, error) {
	if len(meta.Operators) == 0 {
		return []layerTransform{defaultMergeOperator()}, nil
	}

	operators := make([]layerTransform, 0, len(meta.Operators))
	hasMerge := false
	for i, opMeta := range meta.Operators {
		fieldPrefix := fmt.Sprintf("operators[%d]", i)
		op, err := buildLayerOperator(opMeta, fieldPrefix)
		if err != nil {
			return nil, err
		}
		if op.kind == transformKindMerge {
			hasMerge = true
		}
		operators = append(operators, op)
	}
	if !hasMerge {
		operators = append(operators, defaultMergeOperator())
	}

	return operators, nil
}

func buildLayerOperator(meta layerOperatorMetadata, fieldPrefix string) (layerTransform, error) {
	if meta.Kind == "" {
		return layerTransform{}, fmt.Errorf("invalid %s.kind %q: cannot be empty", fieldPrefix, meta.Kind)
	}

	if meta.Kind == transformKindMerge {
		mergeOp, err := buildMergeOperator(meta, fieldPrefix)
		if err != nil {
			return layerTransform{}, err
		}
		return mergeOp, nil
	}

	transformMeta := layerTransformMetadata{
		Kind:        meta.Kind,
		Source:      meta.Source,
		Target:      meta.Target,
		ListFilter:  meta.ListFilter,
		ListExtract: meta.ListExtract,
		ReplaceVals: meta.ReplaceVals,
	}
	return buildLayerTransform(transformMeta, fieldPrefix)
}

func buildMergeOperator(meta layerOperatorMetadata, fieldPrefix string) (layerTransform, error) {
	source, err := parseOperatorSource(meta.Source, fieldPrefix, transformSourceLayer, sourcePathOptional)
	if err != nil {
		return layerTransform{}, err
	}

	strategy, err := buildLayerMergeStrategy(meta.Merge)
	if err != nil {
		return layerTransform{}, fmt.Errorf("invalid %s.merge: %w", fieldPrefix, err)
	}

	op := layerTransform{
		kind:          transformKindMerge,
		sourceFrom:    source.from,
		sourceFile:    source.file,
		merge:         strategy,
		sourcePath:    source.path,
		hasSourcePath: source.hasPath,
	}

	return op, nil
}

func buildLayerTransform(meta layerTransformMetadata, fieldPrefix string) (layerTransform, error) {
	if meta.Kind != transformKindListFilter && meta.Kind != transformKindListExtract && meta.Kind != transformKindReplaceVals {
		return layerTransform{}, fmt.Errorf("invalid %s.kind %q: supported values: list_filter, list_extract, replace_values", fieldPrefix, meta.Kind)
	}

	source, err := parseOperatorSource(meta.Source, fieldPrefix, transformSourceFile, sourcePathRequired)
	if err != nil {
		return layerTransform{}, err
	}

	targetPath, err := parseTargetPath(meta.Target.Path, meta.Source.Path, fieldPrefix)
	if err != nil {
		return layerTransform{}, err
	}

	transform := layerTransform{
		kind:          meta.Kind,
		sourceFrom:    source.from,
		sourceFile:    source.file,
		sourcePath:    source.path,
		hasSourcePath: source.hasPath,
		targetPath:    targetPath,
	}

	switch meta.Kind {
	case transformKindListFilter:
		filter, err := buildListFilter(meta.ListFilter, fieldPrefix)
		if err != nil {
			return layerTransform{}, err
		}
		transform.listFilter = filter
	case transformKindListExtract:
		extract, err := buildListExtract(meta.ListExtract, fieldPrefix)
		if err != nil {
			return layerTransform{}, err
		}
		transform.listExtract = extract
	case transformKindReplaceVals:
		replaceVals, err := buildReplaceValues(meta.ReplaceVals, fieldPrefix)
		if err != nil {
			return layerTransform{}, err
		}
		transform.replaceVals = replaceVals
	}

	return transform, nil
}

func buildReplaceValues(meta layerReplaceValuesMetadata, fieldPrefix string) (layerReplaceValues, error) {
	if meta.Old == "" {
		return layerReplaceValues{}, fmt.Errorf("invalid %s.replace_values.old: cannot be empty", fieldPrefix)
	}

	return layerReplaceValues{
		old:           meta.Old,
		new:           meta.New,
		recursive:     meta.Recursive,
		printOriginal: meta.PrintOriginal,
	}, nil
}

func buildListExtract(meta layerListExtractMetadata, fieldPrefix string) (layerListExtract, error) {
	extractPath, err := splitDotPath(meta.ExtractPath)
	if err != nil {
		return layerListExtract{}, fmt.Errorf("invalid %s.list_extract.extract_path %q: %w", fieldPrefix, meta.ExtractPath, err)
	}

	filterConfig, err := parseRegexFilterConfig(
		meta.Include,
		meta.Exclude,
		meta.IncludeMode,
		fmt.Sprintf("%s.list_extract", fieldPrefix),
	)
	if err != nil {
		return layerListExtract{}, err
	}

	return layerListExtract{
		extractPath: extractPath,
		include:     filterConfig.include,
		exclude:     filterConfig.exclude,
		includeMode: filterConfig.includeMode,
	}, nil
}

func buildListFilter(meta layerListFilterMetadata, fieldPrefix string) (layerListFilter, error) {
	filterConfig, err := parseRegexFilterConfig(
		meta.Include,
		meta.Exclude,
		meta.IncludeMode,
		fmt.Sprintf("%s.list_filter", fieldPrefix),
	)
	if err != nil {
		return layerListFilter{}, err
	}

	matchPath, err := parseOptionalPath(meta.MatchPath, fmt.Sprintf("%s.list_filter.match_path", fieldPrefix))
	if err != nil {
		return layerListFilter{}, err
	}

	return layerListFilter{
		matchPath:   matchPath,
		include:     filterConfig.include,
		exclude:     filterConfig.exclude,
		includeMode: filterConfig.includeMode,
	}, nil
}

func parseRegexFilterConfig(include []string, exclude []string, includeModeRaw string, fieldPrefix string) (regexFilterConfig, error) {
	includeRegex, err := compileRegexList(include, fmt.Sprintf("%s.include", fieldPrefix))
	if err != nil {
		return regexFilterConfig{}, err
	}

	excludeRegex, err := compileRegexList(exclude, fmt.Sprintf("%s.exclude", fieldPrefix))
	if err != nil {
		return regexFilterConfig{}, err
	}

	mode, err := parseIncludeMode(includeModeRaw, fmt.Sprintf("%s.include_mode", fieldPrefix))
	if err != nil {
		return regexFilterConfig{}, err
	}

	return regexFilterConfig{
		include:     includeRegex,
		exclude:     excludeRegex,
		includeMode: mode,
	}, nil
}

func parseOptionalPath(rawPath string, fieldName string) ([]string, error) {
	if rawPath == "" {
		return nil, nil
	}

	path, err := splitDotPath(rawPath)
	if err != nil {
		return nil, fmt.Errorf("invalid %s %q: %w", fieldName, rawPath, err)
	}

	return path, nil
}

func parseTargetPath(targetPathRaw string, sourcePathRaw string, fieldPrefix string) ([]string, error) {
	if targetPathRaw == "" {
		targetPathRaw = sourcePathRaw
	}

	targetPath, err := splitDotPath(targetPathRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid %s.target.path %q: %w", fieldPrefix, targetPathRaw, err)
	}

	return targetPath, nil
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

type sourcePathRequirement int

const (
	sourcePathOptional sourcePathRequirement = iota
	sourcePathRequired
)

func parseOperatorSource(meta layerTransformSource, fieldPrefix string, defaultFrom string, pathRequirement sourcePathRequirement) (parsedOperatorSource, error) {
	from := meta.From
	if from == "" {
		from = defaultFrom
	}
	if from != transformSourceFile && from != transformSourceState && from != transformSourceLayer {
		return parsedOperatorSource{}, fmt.Errorf("invalid %s.source.from %q: supported values: file, state, layer", fieldPrefix, meta.From)
	}
	if from == transformSourceFile && meta.File == "" {
		return parsedOperatorSource{}, fmt.Errorf("invalid %s.source.file: cannot be empty when source.from=file", fieldPrefix)
	}
	if from != transformSourceFile && meta.File != "" {
		return parsedOperatorSource{}, fmt.Errorf("invalid %s.source.file: must be empty when source.from is not file", fieldPrefix)
	}

	if pathRequirement == sourcePathRequired && meta.Path == "" {
		return parsedOperatorSource{}, fmt.Errorf("invalid %s.source.path %q: path cannot be empty", fieldPrefix, meta.Path)
	}
	if meta.Path == "" {
		return parsedOperatorSource{from: from, file: meta.File}, nil
	}

	path, err := splitDotPath(meta.Path)
	if err != nil {
		return parsedOperatorSource{}, fmt.Errorf("invalid %s.source.path %q: %w", fieldPrefix, meta.Path, err)
	}

	return parsedOperatorSource{
		from:    from,
		file:    meta.File,
		path:    path,
		hasPath: true,
	}, nil
}

func parseIncludeMode(raw string, fieldName string) (includeMode, error) {
	if raw == "" {
		return includeModeAny, nil
	}

	mode := includeMode(raw)
	if mode != includeModeAny && mode != includeModeAll {
		return "", fmt.Errorf("invalid %s %q: supported values: any, all", fieldName, raw)
	}

	return mode, nil
}

func (c *Compose) applyLayerOperator(layer map[string]interface{}, operator layerTransform, state map[string]interface{}) (map[string]interface{}, map[string]interface{}, error) {
	if layer == nil {
		layer = map[string]interface{}{}
	}
	if state == nil {
		state = map[string]interface{}{}
	}

	input, err := c.resolveOperatorInput(operator, layer, state)
	if err != nil {
		return nil, nil, err
	}

	result, err := c.executeOperator(operator, input, state)
	if err != nil {
		return nil, nil, err
	}
	state = result.state

	if !result.writeTarget {
		return layer, state, nil
	}

	if err := setMapValueAtPath(layer, operator.targetPath, result.output); err != nil {
		return nil, nil, err
	}

	return layer, state, nil
}

func (c *Compose) resolveOperatorInput(operator layerTransform, layer map[string]interface{}, state map[string]interface{}) (interface{}, error) {
	sourceData, err := c.readOperatorSourceData(operator, layer, state)
	if err != nil {
		return nil, err
	}

	if !operator.hasSourcePath {
		return sourceData, nil
	}

	input, ok := getValueAtPath(sourceData, operator.sourcePath)
	if !ok {
		return nil, fmt.Errorf("source path %q not found", normalizePath(operator.sourcePath))
	}

	return input, nil
}

func (c *Compose) readOperatorSourceData(operator layerTransform, layer map[string]interface{}, state map[string]interface{}) (interface{}, error) {
	switch operator.sourceFrom {
	case transformSourceState:
		return state, nil
	case transformSourceFile:
		return c.readSourceYAML(operator.sourceFile)
	case transformSourceLayer:
		return layer, nil
	default:
		return nil, fmt.Errorf("unsupported operator source.from %q", operator.sourceFrom)
	}
}

func (c *Compose) executeOperator(operator layerTransform, input interface{}, state map[string]interface{}) (operatorExecutionResult, error) {
	switch operator.kind {
	case transformKindMerge:
		return executeMergeOperator(input, operator, state)
	case transformKindListFilter:
		return executeListFilterOperator(input, operator, state)
	case transformKindListExtract:
		return executeListExtractOperator(input, operator, state)
	case transformKindReplaceVals:
		return c.executeReplaceValuesOperator(input, operator, state)
	default:
		return operatorExecutionResult{}, fmt.Errorf("unsupported operator kind %q", operator.kind)
	}
}

func executeMergeOperator(input interface{}, operator layerTransform, state map[string]interface{}) (operatorExecutionResult, error) {
	inputMap, err := requireMapInput(input, operator.sourcePath)
	if err != nil {
		return operatorExecutionResult{}, err
	}

	return operatorExecutionResult{
		state: mergeMapsWithStrategy(state, inputMap, operator.merge, nil),
	}, nil
}

func executeListFilterOperator(input interface{}, operator layerTransform, state map[string]interface{}) (operatorExecutionResult, error) {
	return executeListOutputOperator(input, operator.sourcePath, state, func(inputList []interface{}) (interface{}, error) {
		return applyListFilter(inputList, operator.listFilter)
	})
}

func executeListExtractOperator(input interface{}, operator layerTransform, state map[string]interface{}) (operatorExecutionResult, error) {
	return executeListOutputOperator(input, operator.sourcePath, state, func(inputList []interface{}) (interface{}, error) {
		return applyListExtract(inputList, operator.listExtract)
	})
}

func executeListOutputOperator(input interface{}, sourcePath []string, state map[string]interface{}, apply func([]interface{}) (interface{}, error)) (operatorExecutionResult, error) {
	inputList, err := requireListInput(input, sourcePath)
	if err != nil {
		return operatorExecutionResult{}, err
	}

	output, err := apply(inputList)
	if err != nil {
		return operatorExecutionResult{}, err
	}

	return newWriteTargetResult(state, output), nil
}

func (c *Compose) executeReplaceValuesOperator(input interface{}, operator layerTransform, state map[string]interface{}) (operatorExecutionResult, error) {
	output, originals := applyReplaceValues(input, operator.replaceVals)
	if err := c.printReplacedOriginals(originals, operator.replaceVals.printOriginal); err != nil {
		return operatorExecutionResult{}, err
	}

	return newWriteTargetResult(state, output), nil
}

func (c *Compose) printReplacedOriginals(originals []string, enabled bool) error {
	if !enabled {
		return nil
	}

	for _, original := range originals {
		if _, err := fmt.Fprintln(c.logOut, original); err != nil {
			return fmt.Errorf("print replaced original value: %w", err)
		}
	}

	return nil
}

func newWriteTargetResult(state map[string]interface{}, output interface{}) operatorExecutionResult {
	return operatorExecutionResult{
		state:       state,
		output:      output,
		writeTarget: true,
	}
}

func requireListInput(input interface{}, sourcePath []string) ([]interface{}, error) {
	inputList, ok := input.([]interface{})
	if !ok {
		return nil, fmt.Errorf("source path %q must resolve to a list", normalizePath(sourcePath))
	}

	return inputList, nil
}

func requireMapInput(input interface{}, sourcePath []string) (map[string]interface{}, error) {
	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("source path %q must resolve to an object", normalizePath(sourcePath))
	}

	return inputMap, nil
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

		if !shouldKeepCandidate(candidate, filter.include, filter.exclude, filter.includeMode) {
			continue
		}
		out = append(out, item)
	}

	return out, nil
}

func applyListExtract(input []interface{}, extract layerListExtract) ([]interface{}, error) {
	out := make([]interface{}, 0, len(input))
	for i, item := range input {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: expected object list item", i)
		}

		v, ok := getValueAtPath(m, extract.extractPath)
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: extract_path %q not found", i, normalizePath(extract.extractPath))
		}

		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("invalid list item at index %d: extract_path %q must resolve to string", i, normalizePath(extract.extractPath))
		}

		if !shouldKeepCandidate(s, extract.include, extract.exclude, extract.includeMode) {
			continue
		}

		out = append(out, s)
	}

	return out, nil
}

func applyReplaceValues(input interface{}, replace layerReplaceValues) (interface{}, []string) {
	return replaceValues(input, replace.old, replace.new, replace.recursive)
}

func replaceValues(input interface{}, old string, new string, recursive bool) (interface{}, []string) {
	s, ok := input.(string)
	if ok {
		replaced := strings.ReplaceAll(s, old, new)
		if replaced != s {
			return replaced, []string{s}
		}
		return replaced, nil
	}

	m, ok := input.(map[string]interface{})
	if ok {
		out := make(map[string]interface{}, len(m))
		originals := make([]string, 0)
		for k, v := range m {
			if recursive {
				replaced, childOriginals := replaceValues(v, old, new, true)
				out[k] = replaced
				originals = append(originals, childOriginals...)
				continue
			}
			if sv, isString := v.(string); isString {
				replaced := strings.ReplaceAll(sv, old, new)
				if replaced != sv {
					originals = append(originals, sv)
				}
				out[k] = replaced
				continue
			}
			out[k] = v
		}
		return out, originals
	}

	list, ok := input.([]interface{})
	if ok {
		out := make([]interface{}, len(list))
		originals := make([]string, 0)
		for i, v := range list {
			if recursive {
				replaced, childOriginals := replaceValues(v, old, new, true)
				out[i] = replaced
				originals = append(originals, childOriginals...)
				continue
			}
			if sv, isString := v.(string); isString {
				replaced := strings.ReplaceAll(sv, old, new)
				if replaced != sv {
					originals = append(originals, sv)
				}
				out[i] = replaced
				continue
			}
			out[i] = v
		}
		return out, originals
	}

	return input, nil
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

func shouldKeepCandidate(candidate string, include []*regexp.Regexp, exclude []*regexp.Regexp, mode includeMode) bool {
	if !matchesInclude(candidate, include, mode) {
		return false
	}
	if matchesAny(candidate, exclude) {
		return false
	}
	return true
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
	var raw map[string]interface{}
	if err := doc.Decode(&raw); err != nil {
		return layerMetadata{}, fmt.Errorf("failed to decode metadata document: %w", err)
	}

	forbidden := []string{"merge", "transform", "transforms"}
	for _, key := range forbidden {
		if _, ok := raw[key]; ok {
			return layerMetadata{}, fmt.Errorf("legacy metadata field %q is not supported; use operators", key)
		}
	}

	var meta layerMetadata
	if err := doc.Decode(&meta); err != nil {
		return layerMetadata{}, fmt.Errorf("failed to decode metadata document: %w", err)
	}
	return meta, nil
}

func buildLayerMergeStrategy(meta mergeMetadata) (layerMergeStrategy, error) {
	strategy := layerMergeStrategy{
		defaults: defaultMergeStrategy,
		paths:    map[string]mergeStrategy{},
	}

	var err error
	strategy.defaults, err = applyMetadataStrategy(strategy.defaults, meta.Defaults)
	if err != nil {
		return layerMergeStrategy{}, fmt.Errorf("invalid merge.defaults: %w", err)
	}

	for rawPath, override := range meta.Paths {
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
		switch typed := cur.(type) {
		case map[string]interface{}:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			cur = next
		case []interface{}:
			if index, ok := parsePathIndex(segment); ok {
				if index < 0 || index >= len(typed) {
					return nil, false
				}
				cur = typed[index]
				continue
			}

			selectorKey, selectorValue, ok := parsePathSelector(segment)
			if !ok {
				return nil, false
			}
			index, ok := findArrayObjectBySelector(typed, selectorKey, selectorValue)
			if !ok {
				return nil, false
			}
			cur = typed[index]
		default:
			return nil, false
		}
	}

	return cur, true
}

func setMapValueAtPath(root map[string]interface{}, path []string, value interface{}) error {
	if len(path) == 0 {
		return fmt.Errorf("path cannot be empty")
	}

	var cur interface{} = root
	for i := 0; i < len(path)-1; i++ {
		segment := path[i]
		switch typed := cur.(type) {
		case map[string]interface{}:
			next, ok := typed[segment]
			if !ok {
				if _, isIndex := parsePathIndex(path[i+1]); isIndex {
					return fmt.Errorf("target path %q is not writable: segment %q must be an array", normalizePath(path), normalizePath(path[:i+1]))
				}
				child := map[string]interface{}{}
				typed[segment] = child
				cur = child
				continue
			}
			cur = next
		case []interface{}:
			if index, ok := parsePathIndex(segment); ok {
				if index < 0 || index >= len(typed) {
					return fmt.Errorf("target path %q is not writable: index %d out of range at segment %q", normalizePath(path), index, normalizePath(path[:i+1]))
				}
				cur = typed[index]
				continue
			}

			selectorKey, selectorValue, ok := parsePathSelector(segment)
			if !ok {
				return fmt.Errorf("target path %q is not writable: segment %q must be an array index or selector", normalizePath(path), normalizePath(path[:i+1]))
			}

			index, err := findUniqueArrayObjectBySelector(typed, selectorKey, selectorValue)
			if err != nil {
				return fmt.Errorf("target path %q is not writable: %w", normalizePath(path), err)
			}
			cur = typed[index]
		default:
			return fmt.Errorf("target path %q is not writable: segment %q is not an object or array", normalizePath(path), normalizePath(path[:i+1]))
		}
	}

	lastSegment := path[len(path)-1]
	switch typed := cur.(type) {
	case map[string]interface{}:
		typed[lastSegment] = value
		return nil
	case []interface{}:
		if index, ok := parsePathIndex(lastSegment); ok {
			if index < 0 || index >= len(typed) {
				return fmt.Errorf("target path %q is not writable: final index %d out of range", normalizePath(path), index)
			}
			typed[index] = value
			return nil
		}

		selectorKey, selectorValue, ok := parsePathSelector(lastSegment)
		if !ok {
			return fmt.Errorf("target path %q is not writable: final segment %q must be an array index or selector", normalizePath(path), normalizePath(path))
		}
		index, err := findUniqueArrayObjectBySelector(typed, selectorKey, selectorValue)
		if err != nil {
			return fmt.Errorf("target path %q is not writable: %w", normalizePath(path), err)
		}
		typed[index] = value
		return nil
	default:
		return fmt.Errorf("target path %q is not writable: parent is not an object or array", normalizePath(path))
	}
}

func parsePathIndex(segment string) (int, bool) {
	index, err := strconv.Atoi(segment)
	if err != nil {
		return 0, false
	}
	return index, true
}

func parsePathSelector(segment string) (string, string, bool) {
	key, value, ok := strings.Cut(segment, "=")
	if !ok {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", "", false
	}

	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) || (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
		if len(value) < 2 {
			return "", "", false
		}

		if strings.HasPrefix(value, "'") {
			inner := value[1 : len(value)-1]
			inner = strings.ReplaceAll(inner, `\'`, `'`)
			inner = strings.ReplaceAll(inner, `\\`, `\`)
			return key, inner, true
		}

		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return "", "", false
		}
		return key, unquoted, true
	}

	return key, value, true
}

func findArrayObjectBySelector(items []interface{}, key string, expected string) (int, bool) {
	index, err := findUniqueArrayObjectBySelector(items, key, expected)
	if err != nil {
		return 0, false
	}
	return index, true
}

func findUniqueArrayObjectBySelector(items []interface{}, key string, expected string) (int, error) {
	matches := make([]int, 0)
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return 0, fmt.Errorf("selector [%s=%s] requires object array items, got %T at index %d", key, expected, item, i)
		}
		raw, ok := m[key]
		if !ok {
			continue
		}
		actual, ok := raw.(string)
		if !ok {
			return 0, fmt.Errorf("selector [%s=%s] requires string field %q, got %T at index %d", key, expected, key, raw, i)
		}
		if actual == expected {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return 0, fmt.Errorf("selector [%s=%s] matched no array item", key, expected)
	}
	if len(matches) > 1 {
		return 0, fmt.Errorf("selector [%s=%s] matched multiple array items", key, expected)
	}

	return matches[0], nil
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

	expandedParts, err := expandBracketPathSegments(parts)
	if err != nil {
		return nil, err
	}

	return expandedParts, nil
}

func expandBracketPathSegments(parts []string) ([]string, error) {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		expanded, err := expandBracketPathSegment(part)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded...)
	}
	return out, nil
}

func expandBracketPathSegment(segment string) ([]string, error) {
	out := make([]string, 0, 1)
	var token strings.Builder
	afterIndex := false

	for i := 0; i < len(segment); {
		ch := segment[i]
		if ch != '[' {
			if afterIndex {
				return nil, fmt.Errorf("invalid bracket path segment %q", segment)
			}
			token.WriteByte(ch)
			i++
			continue
		}

		if token.Len() > 0 {
			out = append(out, token.String())
			token.Reset()
		}

		end := i + 1
		for end < len(segment) && segment[end] != ']' {
			end++
		}
		if end >= len(segment) {
			return nil, fmt.Errorf("invalid bracket path segment %q", segment)
		}

		indexRaw := segment[i+1 : end]
		if indexRaw == "" {
			return nil, fmt.Errorf("invalid bracket path segment %q", segment)
		}
		if _, err := strconv.Atoi(indexRaw); err != nil {
			if _, _, ok := parsePathSelector(indexRaw); !ok {
				return nil, fmt.Errorf("invalid bracket path segment %q", segment)
			}
		}

		out = append(out, indexRaw)
		afterIndex = true
		i = end + 1
	}

	if token.Len() > 0 {
		out = append(out, token.String())
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty segment")
	}

	return out, nil
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
