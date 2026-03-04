package compose

import (
	"fmt"
	"strconv"
	"strings"
)

// NewLayerComparator returns a sort.SliceStable-compatible comparator that
// orders layer filenames by their numeric prefix, then lexicographically by
// name when the prefix is equal.
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

// part splits a layer filename at the first "-" and returns (prefix, rest).
// If there is no "-", the entire string is returned as prefix with an empty rest.
func part(s string) (string, string) {
	left, right, ok := strings.Cut(s, "-")
	if !ok {
		return s, ""
	}
	return left, right
}

// validateLayerName returns an error when layer does not follow the required
// "<numeric-order>-<name>" naming convention.
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
