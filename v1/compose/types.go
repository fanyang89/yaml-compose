package compose

import (
	"regexp"
)

type marshalFunc func(any) ([]byte, error)

const yamlOutputIndentSpaces = 2

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
