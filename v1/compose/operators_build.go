package compose

import (
	"fmt"
	"regexp"
)

type regexFilterConfig struct {
	include     []*regexp.Regexp
	exclude     []*regexp.Regexp
	includeMode includeMode
}

type sourcePathRequirement int

const (
	sourcePathOptional sourcePathRequirement = iota
	sourcePathRequired
)

type parsedOperatorTarget struct {
	path           []string
	merge          layerMergeStrategy
	ignoreNotFound bool
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
		ListRemove:  meta.ListRemove,
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
	if meta.Kind != transformKindListFilter && meta.Kind != transformKindListExtract && meta.Kind != transformKindListRemove && meta.Kind != transformKindReplaceVals {
		return layerTransform{}, fmt.Errorf("invalid %s.kind %q: supported values: list_filter, list_extract, list_remove, replace_values", fieldPrefix, meta.Kind)
	}

	source, err := parseOperatorSource(meta.Source, fieldPrefix, transformSourceFile, sourcePathRequired)
	if err != nil {
		return layerTransform{}, err
	}

	target, err := parseOperatorTarget(
		meta.Target,
		meta.Source.Path,
		fieldPrefix,
		meta.Kind == transformKindListFilter || meta.Kind == transformKindListExtract,
		meta.Kind == transformKindListExtract,
	)
	if err != nil {
		return layerTransform{}, err
	}

	transform := layerTransform{
		kind:                 meta.Kind,
		sourceFrom:           source.from,
		sourceFile:           source.file,
		sourcePath:           source.path,
		hasSourcePath:        source.hasPath,
		targetPath:           target.path,
		targetMerge:          target.merge,
		ignoreTargetNotFound: target.ignoreNotFound,
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
	case transformKindListRemove:
		listRemove, err := buildListRemove(meta.ListRemove, fieldPrefix)
		if err != nil {
			return layerTransform{}, err
		}
		transform.listRemove = listRemove
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

func buildListRemove(meta layerListRemoveMetadata, fieldPrefix string) (layerListRemove, error) {
	matchPath, err := splitDotPath(meta.MatchPath)
	if err != nil {
		return layerListRemove{}, fmt.Errorf("invalid %s.list_remove.match_path %q: %w", fieldPrefix, meta.MatchPath, err)
	}

	predicate, predicateValue, err := parseListRemoveCondition(meta.When, fieldPrefix)
	if err != nil {
		return layerListRemove{}, err
	}

	removeMode := listRemoveAll
	if meta.Remove != "" {
		removeMode = listRemoveMode(meta.Remove)
	}
	if removeMode != listRemoveAll && removeMode != listRemoveSingle {
		return layerListRemove{}, fmt.Errorf("invalid %s.list_remove.remove %q: supported values: all, single", fieldPrefix, meta.Remove)
	}

	return layerListRemove{matchPath: matchPath, remove: removeMode, predicate: predicate, value: predicateValue}, nil
}

func parseListRemoveCondition(meta layerListRemoveWhenMetadata, fieldPrefix string) (listRemovePredicate, any, error) {
	configured := 0
	predicate := listRemovePredicate("")
	var value any

	if meta.IsEmpty {
		configured++
		predicate = listRemovePredicateIsEmpty
	}
	if meta.Equals != nil {
		configured++
		predicate = listRemovePredicateEquals
		value = meta.Equals
	}
	if meta.NotEquals != nil {
		configured++
		predicate = listRemovePredicateNotEquals
		value = meta.NotEquals
	}
	if meta.Has != nil {
		configured++
		predicate = listRemovePredicateHas
		value = meta.Has
	}

	if configured == 0 {
		return "", nil, fmt.Errorf("invalid %s.list_remove.when: set one of is_empty=true, equals, not_equals, has", fieldPrefix)
	}
	if configured > 1 {
		return "", nil, fmt.Errorf("invalid %s.list_remove.when: only one condition is supported", fieldPrefix)
	}

	return predicate, value, nil
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

	rewrite, err := parseListFilterRewrite(meta.Rewrite, matchPath, fieldPrefix)
	if err != nil {
		return layerListFilter{}, err
	}

	return layerListFilter{
		matchPath:   matchPath,
		include:     filterConfig.include,
		exclude:     filterConfig.exclude,
		includeMode: filterConfig.includeMode,
		rewrite:     rewrite,
	}, nil
}

func parseListFilterRewrite(meta layerListFilterRewriteMetadata, matchPath []string, fieldPrefix string) (*layerListFilterRewrite, error) {
	configured := meta.Prefix != "" || meta.Path != ""
	if !configured {
		return nil, nil
	}

	if meta.Prefix == "" {
		return nil, fmt.Errorf("invalid %s.list_filter.rewrite.prefix: cannot be empty when rewrite is configured", fieldPrefix)
	}

	rewritePath := matchPath
	if meta.Path != "" {
		parsedPath, err := splitDotPath(meta.Path)
		if err != nil {
			return nil, fmt.Errorf("invalid %s.list_filter.rewrite.path %q: %w", fieldPrefix, meta.Path, err)
		}
		rewritePath = parsedPath
	}

	return &layerListFilterRewrite{prefix: meta.Prefix, path: rewritePath}, nil
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

func parseOperatorTarget(meta layerTransformTarget, sourcePathRaw string, fieldPrefix string, supportsListStrategy bool, supportsIgnoreNotFound bool) (parsedOperatorTarget, error) {
	hasLegacyList := meta.List != ""
	hasMerge := meta.Merge.Defaults.Map != "" || meta.Merge.Defaults.List != "" || len(meta.Merge.Paths) > 0

	if hasLegacyList && hasMerge {
		return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target: target.list and target.merge cannot be used together", fieldPrefix)
	}

	if hasLegacyList {
		return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.list: use %s.target.merge.defaults.list instead", fieldPrefix, fieldPrefix)
	}

	if !supportsListStrategy && hasMerge {
		return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.merge: only list_filter and list_extract support target.merge", fieldPrefix)
	}
	if meta.IgnoreNotFound && !supportsIgnoreNotFound {
		return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.ignore_not_found: only list_extract supports target.ignore_not_found", fieldPrefix)
	}

	targetMerge := layerMergeStrategy{defaults: defaultMergeStrategy}
	if hasMerge {
		if meta.Merge.Defaults.Map != "" {
			return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.merge.defaults.map: target.merge only supports defaults.list", fieldPrefix)
		}
		if len(meta.Merge.Paths) > 0 {
			return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.merge.paths: target.merge only supports defaults.list", fieldPrefix)
		}

		strategy, err := buildLayerMergeStrategy(meta.Merge)
		if err != nil {
			return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.merge: %w", fieldPrefix, err)
		}
		targetMerge = strategy
	}

	targetPathRaw := meta.Path
	if targetPathRaw == "" {
		targetPathRaw = sourcePathRaw
	}

	targetPath, err := splitDotPath(targetPathRaw)
	if err != nil {
		return parsedOperatorTarget{}, fmt.Errorf("invalid %s.target.path %q: %w", fieldPrefix, targetPathRaw, err)
	}

	return parsedOperatorTarget{path: targetPath, merge: targetMerge, ignoreNotFound: meta.IgnoreNotFound}, nil
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
