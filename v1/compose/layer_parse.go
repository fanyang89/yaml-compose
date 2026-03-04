package compose

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// parseLayer reads a raw layer file (one or two YAML documents) and returns
// the data map together with the list of operators to apply.
func parseLayer(in []byte) (map[string]any, []layerTransform, error) {
	docs, err := decodeYAMLDocuments(in)
	if err != nil {
		return nil, nil, err
	}

	switch len(docs) {
	case 0:
		return map[string]any{}, []layerTransform{defaultMergeOperator()}, nil
	case 1:
		data, err := decodeYAMLMap(docs[0])
		if err != nil {
			return nil, nil, err
		}

		if rawOperators, hasOperators := data["operators"]; hasOperators && looksLikeOperatorMetadata(rawOperators) {
			if len(data) == 1 {
				meta, err := decodeLayerMetadata(docs[0])
				if err != nil {
					return nil, nil, err
				}
				operators, err := buildLayerOperators(meta)
				if err != nil {
					return nil, nil, err
				}
				return map[string]any{}, operators, nil
			}

			return nil, nil, fmt.Errorf("layer with operators metadata must use two YAML documents separated by ---")
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

func looksLikeOperatorMetadata(raw any) bool {
	ops, ok := raw.([]any)
	if !ok || len(ops) == 0 {
		return false
	}

	for _, op := range ops {
		m, ok := op.(map[string]any)
		if !ok {
			return false
		}
		kind, ok := m["kind"].(string)
		if !ok || kind == "" {
			return false
		}
	}

	return true
}

// defaultMergeOperator returns the implicit merge operator used when no
// operator metadata is present in a layer file.
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

// buildLayerOperators converts the operator metadata slice into runtime
// layerTransform values.  A default merge operator is appended automatically
// when none of the declared operators is of kind "merge".
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

func decodeYAMLMap(doc *yaml.Node) (map[string]any, error) {
	var m map[string]any
	if err := doc.Decode(&m); err != nil {
		return nil, fmt.Errorf("expected YAML mapping document: %w", err)
	}
	if m == nil {
		return map[string]any{}, nil
	}
	return m, nil
}

func decodeLayerMetadata(doc *yaml.Node) (layerMetadata, error) {
	var raw map[string]any
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
