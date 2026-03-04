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
	ListRemove  layerListRemoveMetadata    `yaml:"list_remove"`
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
	ListRemove  layerListRemoveMetadata    `yaml:"list_remove"`
	ReplaceVals layerReplaceValuesMetadata `yaml:"replace_values"`
}

type layerTransformSource struct {
	From string `yaml:"from"`
	File string `yaml:"file"`
	Path string `yaml:"path"`
}

type layerTransformTarget struct {
	Path           string        `yaml:"path"`
	Merge          mergeMetadata `yaml:"merge"`
	List           string        `yaml:"list"`
	IgnoreNotFound bool          `yaml:"ignore_not_found"`
}

type layerListFilterMetadata struct {
	MatchPath   string                         `yaml:"match_path"`
	Include     []string                       `yaml:"include"`
	Exclude     []string                       `yaml:"exclude"`
	IncludeMode string                         `yaml:"include_mode"`
	Rewrite     layerListFilterRewriteMetadata `yaml:"rewrite"`
}

type layerListFilterRewriteMetadata struct {
	Prefix string `yaml:"prefix"`
	Path   string `yaml:"path"`
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

type layerListRemoveMetadata struct {
	MatchPath string                      `yaml:"match_path"`
	When      layerListRemoveWhenMetadata `yaml:"when"`
	Remove    string                      `yaml:"remove"`
}

type layerListRemoveWhenMetadata struct {
	IsEmpty   bool `yaml:"is_empty"`
	Equals    any  `yaml:"equals"`
	NotEquals any  `yaml:"not_equals"`
	Has       any  `yaml:"has"`
}

type layerTransform struct {
	kind                 string
	sourceFrom           string
	sourceFile           string
	sourcePath           []string
	hasSourcePath        bool
	targetPath           []string
	targetMerge          layerMergeStrategy
	ignoreTargetNotFound bool
	listFilter           layerListFilter
	listExtract          layerListExtract
	listRemove           layerListRemove
	replaceVals          layerReplaceValues
	merge                layerMergeStrategy
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
	rewrite     *layerListFilterRewrite
}

type layerListFilterRewrite struct {
	prefix string
	path   []string
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

type listRemoveMode string

const (
	listRemoveAll    listRemoveMode = "all"
	listRemoveSingle listRemoveMode = "single"
)

type layerListRemove struct {
	matchPath []string
	remove    listRemoveMode
	predicate listRemovePredicate
	value     any
}

type listRemovePredicate string

const (
	listRemovePredicateIsEmpty   listRemovePredicate = "is_empty"
	listRemovePredicateEquals    listRemovePredicate = "equals"
	listRemovePredicateNotEquals listRemovePredicate = "not_equals"
	listRemovePredicateHas       listRemovePredicate = "has"
)

type includeMode string

const (
	transformKindMerge       = "merge"
	transformKindListFilter  = "list_filter"
	transformKindListExtract = "list_extract"
	transformKindListRemove  = "list_remove"
	transformKindReplaceVals = "replace_values"

	transformSourceFile  = "file"
	transformSourceState = "state"
	transformSourceLayer = "layer"

	includeModeAny includeMode = "any"
	includeModeAll includeMode = "all"
)
