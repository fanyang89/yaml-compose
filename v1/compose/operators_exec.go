package compose

import (
	"errors"
	"fmt"
)

type operatorExecutionResult struct {
	state       map[string]any
	output      any
	writeTarget bool
}

func (c *Compose) applyLayerOperator(layer map[string]any, operator layerTransform, state map[string]any) (map[string]any, map[string]any, error) {
	if layer == nil {
		layer = map[string]any{}
	}
	if state == nil {
		state = map[string]any{}
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

	output, err := resolveTargetOutput(operator, layer, state, result.output)
	if err != nil {
		return nil, nil, err
	}

	if err := setMapValueAtPath(layer, operator.targetPath, output); err != nil {
		if operator.ignoreTargetNotFound && errors.Is(err, errPathSelectorNoMatch) {
			return layer, state, nil
		}
		return nil, nil, err
	}

	return layer, state, nil
}

func resolveTargetOutput(operator layerTransform, layer map[string]any, state map[string]any, output any) (any, error) {
	targetListStrategy := operator.targetMerge.defaults.List
	if targetListStrategy == "" || targetListStrategy == listMergeOverride {
		return output, nil
	}

	outputList, ok := output.([]any)
	if !ok {
		return nil, fmt.Errorf("target.merge.defaults.list %q requires list output, got %T", targetListStrategy, output)
	}

	existing, found := getValueAtPath(layer, operator.targetPath)
	if !found {
		existing, found = getValueAtPath(state, operator.targetPath)
	}
	if !found {
		return outputList, nil
	}

	existingList, ok := existing.([]any)
	if !ok {
		return nil, fmt.Errorf("target path %q must resolve to a list when target.merge.defaults.list=%q", normalizePath(operator.targetPath), targetListStrategy)
	}

	return mergeValue(existingList, outputList, operator.targetMerge, operator.targetPath), nil
}

func (c *Compose) resolveOperatorInput(operator layerTransform, layer map[string]any, state map[string]any) (any, error) {
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

func (c *Compose) readOperatorSourceData(operator layerTransform, layer map[string]any, state map[string]any) (any, error) {
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

func (c *Compose) executeOperator(operator layerTransform, input any, state map[string]any) (operatorExecutionResult, error) {
	switch operator.kind {
	case transformKindMerge:
		return executeMergeOperator(input, operator, state)
	case transformKindListFilter:
		return executeListFilterOperator(input, operator, state)
	case transformKindListExtract:
		return executeListExtractOperator(input, operator, state)
	case transformKindListRemove:
		return executeListRemoveOperator(input, operator, state)
	case transformKindReplaceVals:
		return c.executeReplaceValuesOperator(input, operator, state)
	default:
		return operatorExecutionResult{}, fmt.Errorf("unsupported operator kind %q", operator.kind)
	}
}

func executeMergeOperator(input any, operator layerTransform, state map[string]any) (operatorExecutionResult, error) {
	inputMap, err := requireMapInput(input, operator.sourcePath)
	if err != nil {
		return operatorExecutionResult{}, err
	}

	return operatorExecutionResult{
		state: mergeMapsWithStrategy(state, inputMap, operator.merge, nil),
	}, nil
}

func executeListFilterOperator(input any, operator layerTransform, state map[string]any) (operatorExecutionResult, error) {
	return executeListOutputOperator(input, operator.sourcePath, state, func(inputList []any) (any, error) {
		return applyListFilter(inputList, operator.listFilter)
	})
}

func executeListExtractOperator(input any, operator layerTransform, state map[string]any) (operatorExecutionResult, error) {
	return executeListOutputOperator(input, operator.sourcePath, state, func(inputList []any) (any, error) {
		return applyListExtract(inputList, operator.listExtract)
	})
}

func executeListRemoveOperator(input any, operator layerTransform, state map[string]any) (operatorExecutionResult, error) {
	return executeListOutputOperator(input, operator.sourcePath, state, func(inputList []any) (any, error) {
		return applyListRemove(inputList, operator.listRemove)
	})
}

func executeListOutputOperator(input any, sourcePath []string, state map[string]any, apply func([]any) (any, error)) (operatorExecutionResult, error) {
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

func (c *Compose) executeReplaceValuesOperator(input any, operator layerTransform, state map[string]any) (operatorExecutionResult, error) {
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

func newWriteTargetResult(state map[string]any, output any) operatorExecutionResult {
	return operatorExecutionResult{
		state:       state,
		output:      output,
		writeTarget: true,
	}
}

func requireListInput(input any, sourcePath []string) ([]any, error) {
	inputList, ok := input.([]any)
	if !ok {
		return nil, fmt.Errorf("source path %q must resolve to a list", normalizePath(sourcePath))
	}

	return inputList, nil
}

func requireMapInput(input any, sourcePath []string) (map[string]any, error) {
	inputMap, ok := input.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("source path %q must resolve to an object", normalizePath(sourcePath))
	}

	return inputMap, nil
}
