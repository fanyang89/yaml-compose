package compose

import "fmt"

// mergeMaps deep-merges layer into base using the default merge strategy.
func mergeMaps(base map[string]any, layer map[string]any) map[string]any {
	return mergeMapsWithStrategy(base, layer, layerMergeStrategy{defaults: defaultMergeStrategy}, nil)
}

func mergeMapsWithStrategy(base map[string]any, layer map[string]any, strategy layerMergeStrategy, path []string) map[string]any {
	if base == nil {
		base = map[string]any{}
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

func mergeValue(base any, layer any, strategy layerMergeStrategy, path []string) any {
	baseMap, baseIsMap := base.(map[string]any)
	layerMap, layerIsMap := layer.(map[string]any)
	if baseIsMap && layerIsMap {
		pathStrategy := strategy.resolve(path)
		if pathStrategy.Map == mapMergeOverride {
			return layerMap
		}
		return mergeMapsWithStrategy(baseMap, layerMap, strategy, path)
	}

	baseList, baseIsList := base.([]any)
	layerList, layerIsList := layer.([]any)
	if baseIsList && layerIsList {
		switch strategy.resolve(path).List {
		case listMergeAppend:
			out := make([]any, 0, len(baseList)+len(layerList))
			out = append(out, baseList...)
			out = append(out, layerList...)
			return out
		case listMergePrepend:
			out := make([]any, 0, len(baseList)+len(layerList))
			out = append(out, layerList...)
			out = append(out, baseList...)
			return out
		default:
			return layerList
		}
	}

	return layer
}

// buildLayerMergeStrategy converts the merge metadata from a layer operator
// into the resolved layerMergeStrategy used at runtime.
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

// resolve returns the effective mergeStrategy for the given path, falling back
// to the defaults when no path-specific override is registered.
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

// appendPath returns a new slice with key appended to path, without mutating
// the original slice.
func appendPath(path []string, key string) []string {
	next := make([]string, len(path)+1)
	copy(next, path)
	next[len(path)] = key
	return next
}
