package compose_test

import (
	"bytes"
	"path"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/fanyang89/yaml-compose/v1/compose"
)

func TestLayerSort(t *testing.T) {
	require := require.New(t)

	layers := []string{
		"20-a.yaml",
		"10-b.yaml",
		"1-c.yaml",
		"2-d.yaml",
		"3-e.yaml",
	}

	sort.SliceStable(layers, compose.NewLayerComparator(layers))

	require.Equal([]string{
		"1-c.yaml",
		"2-d.yaml",
		"3-e.yaml",
		"10-b.yaml",
		"20-a.yaml",
	}, layers)
}

func TestLayerSortByNameWhenPrefixEqual(t *testing.T) {
	require := require.New(t)

	layers := []string{
		"1-z.yaml",
		"1-a.yaml",
		"1-b.yaml",
	}

	sort.SliceStable(layers, compose.NewLayerComparator(layers))

	require.Equal([]string{"1-a.yaml", "1-b.yaml", "1-z.yaml"}, layers)
}

func TestLayerSortDoesNotPanicForInvalidPrefix(t *testing.T) {
	require := require.New(t)

	layers := []string{"a-layer.yaml", "2-layer.yaml", "b-layer.yaml"}
	require.NotPanics(func() {
		sort.SliceStable(layers, compose.NewLayerComparator(layers))
	})

	require.Equal([]string{"2-layer.yaml", "a-layer.yaml", "b-layer.yaml"}, layers)
}

func TestLayerComparatorHandlesNonNumericVsNumeric(t *testing.T) {
	require := require.New(t)

	layers := []string{"a-layer.yaml", "1-layer.yaml"}
	cmp := compose.NewLayerComparator(layers)
	require.False(cmp(0, 1))
}

func TestLayerComparatorFallsBackToNameForSameNonNumericPrefix(t *testing.T) {
	require := require.New(t)

	layers := []string{"a-z.yaml", "a-a.yaml"}
	cmp := compose.NewLayerComparator(layers)
	require.False(cmp(0, 1))
}

var baseYAML = `doe: "a deer, a female deer"
ray: "a drop of golden sun"
pi: 3.14159
xmas: true
french-hens:
- 1
- 2
- 3
`

var cYAML = `xmas: false`

var dYAML = `french-hens: [4,5,6]`

var eYAML = ``

func TestCompose(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{
		"1-c.yaml",
		"2-d.yaml",
		"3-e.yaml",
	})

	base := "base.yaml"
	baseDir := base + ".d"
	fs := c.GetFilesystem()
	err := fs.WriteFile(base, []byte(baseYAML), 0755)
	require.NoErrorf(err, "write yaml")
	err = fs.Mkdir(baseDir, 0755)
	require.NoErrorf(err, "create dir")

	err = fs.WriteFile(path.Join(baseDir, "1-c.yaml"), []byte(cYAML), 0755)
	require.NoErrorf(err, "write yaml")
	err = fs.WriteFile(path.Join(baseDir, "2-d.yaml"), []byte(dYAML), 0755)
	require.NoErrorf(err, "write yaml")
	err = fs.WriteFile(path.Join(baseDir, "3-e.yaml"), []byte(eYAML), 0755)
	require.NoErrorf(err, "write yaml")

	r, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(r), &got)
	require.NoError(err)
	require.Equal(false, got["xmas"])
	require.Equal([]interface{}{4, 5, 6}, got["french-hens"])
	require.Equal("a drop of golden sun", got["ray"])
}

func TestComposeDeepMergeWithListOverrideAndNull(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  db:
    host: base
    pool: 10
    ports:
      - 5432
feature: true
keep: value
`
	layer := `app:
  db:
    host: layer
    ports:
      - 5433
feature: null
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	db := app["db"].(map[string]interface{})
	require.Equal("layer", db["host"])
	require.Equal(10, db["pool"])
	require.Equal([]interface{}{5433}, db["ports"])
	require.Nil(got["feature"])
	require.Equal("value", got["keep"])
}

func TestComposeReturnsErrorForInvalidLayerName(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"bad.yaml"})
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/bad.yaml", []byte("xmas: false\n"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "invalid layer file name")
}

func TestComposeListStrategiesByPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  prepend-list: [base-1, base-2]
  append-list: [base-3]
  override-list: [base-4]
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
    merge:
      paths:
        app.prepend-list:
          list: prepend
        app.append-list:
          list: append
        app.override-list:
          list: override
---
app:
  prepend-list: [layer-1]
  append-list: [layer-2]
  override-list: [layer-3]
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"layer-1", "base-1", "base-2"}, app["prepend-list"])
	require.Equal([]interface{}{"base-3", "layer-2"}, app["append-list"])
	require.Equal([]interface{}{"layer-3"}, app["override-list"])
	require.NotContains(got, "merge")
}

func TestComposeMapOverrideByPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  db:
    host: base
    pool: 10
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
    merge:
      paths:
        app.db:
          map: override
---
app:
  db:
    host: layer
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	db := app["db"].(map[string]interface{})
	require.Equal("layer", db["host"])
	require.NotContains(db, "pool")
}

func TestComposeDefaultsAndPathOverride(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `items: [base]
priority-items: [base]
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
    merge:
      defaults:
        list: append
      paths:
        priority-items:
          list: prepend
---
items: [layer]
priority-items: [layer]
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	require.Equal([]interface{}{"base", "layer"}, got["items"])
	require.Equal([]interface{}{"layer", "base"}, got["priority-items"])
}

func TestComposePathSupportsEscapedDotKey(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  db.main:
    ports: [5432]
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
    merge:
      paths:
        app.db\.main.ports:
          list: append
---
app:
  db.main:
    ports: [5433]
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	dbMain := app["db.main"].(map[string]interface{})
	require.Equal([]interface{}{5432, 5433}, dbMain["ports"])
}

func TestComposeReturnsErrorForInvalidMergeStrategy(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `items: [base]
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
    merge:
      defaults:
        list: invalid
---
items: [layer]
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "unsupported list strategy")

}

func TestComposeReturnsErrorForNonNumericLayerPrefix(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"a-layer.yaml"})
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/a-layer.yaml", []byte("xmas: false\n"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "expected numeric order prefix")
}

func TestComposeReturnsErrorWhenBaseReadFails(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", nil)
	_, err := c.Run()
	require.Error(err)
	require.Contains(err.Error(), "failed to read base compose file")
}

func TestComposeReturnsErrorWhenBaseYAMLIsInvalid(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", nil)
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(":"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "failed to parse base compose file")
}

func TestComposeReturnsErrorWhenLayerReadFails(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "failed to read layer compose file")
}

func TestComposeReturnsErrorWhenLayerYAMLIsInvalid(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte("a: [1,2"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "failed to parse layer compose file")
}

func TestComposeWithoutLayersReturnsBaseContent(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", nil)
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)
	require.Equal(true, got["xmas"])
	require.Equal("a drop of golden sun", got["ray"])
}

func TestComposeTransformListFilterStringList(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("configs/base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: [base]
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      include: ["prod-", "cn-"]
      exclude: ["canary$"]
---
app:
  backends: []
`
	source := `inventory:
  backends:
    - prod-a
    - prod-canary
    - cn-main
    - dev-main
`

	err := fs.MkdirAll("configs/base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("configs/base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.WriteFile("configs/base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("configs/source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "cn-main"}, app["backends"])
}

func TestComposeTransformListFilterObjectList(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: []
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      match_path: name
      include: ["prod-"]
---
app:
  backends: []
`
	source := `inventory:
  backends:
    - name: prod-a
      url: https://a
    - name: dev-a
      url: https://dev
    - name: prod-b
      url: https://b
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	backends := app["backends"].([]interface{})
	require.Len(backends, 2)
	first := backends[0].(map[string]interface{})
	second := backends[1].(map[string]interface{})
	require.Equal("prod-a", first["name"])
	require.Equal("prod-b", second["name"])
}

func TestComposeTransformListFilterDefaultsTargetPathToSourcePath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `inventory:
  backends: [base]
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    list_filter:
      include: ["prod-"]
---
inventory:
  backends: []
`
	source := `inventory:
  backends: [prod-a, dev-a]
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	inventory := got["inventory"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a"}, inventory["backends"])
}

func TestComposeTransformListFilterReturnsErrorWhenObjectMatchPathMissing(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: []
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      match_path: name
---
app:
  backends: []
`
	source := `inventory:
  backends:
    - url: https://a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "match_path \"name\" not found")
}

func TestComposeTransformListFilterReturnsErrorWhenObjectListHasNoMatchPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: []
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      include: ["prod-"]
---
app:
  backends: []
`
	source := `inventory:
  backends:
    - name: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "object list requires transform.list_filter.match_path")
}

func TestComposeTransformListFilterReturnsErrorWhenSourcePathIsNotList(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: []
`
	layer := `operators:
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
---
app:
  backends: []
`
	source := `inventory:
  backends: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "must resolve to a list")
}

func TestComposeSupportsMultipleTransformsInSingleLayer(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
  backends: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: name
      include: ["prod-"]
  - kind: list_filter
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends
    list_filter:
      match_path: name
      include: ["prod-"]
---
app:
  backend-names: []
  backends: []
`
	source := `inventory:
  backends:
    - name: prod-a
      url: https://a
    - name: dev-a
      url: https://dev
    - name: prod-b
      url: https://b
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, app["backend-names"])
	backends := app["backends"].([]interface{})
	require.Len(backends, 2)
}

func TestComposeReturnsErrorForUnknownOperatorKind(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: unknown
    source:
      from: layer
---
app:
  backend-names: []
`
	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "supported values")
}

func TestComposeTransformListExtractBuildsStringListFromObjectList(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: meta.name
      include: ["prod-"]
      exclude: ["-canary$"]
---
app:
  backend-names: []
`
	source := `inventory:
  backends:
    - meta:
        name: prod-a
      url: https://a
    - meta:
        name: prod-b
      url: https://b
    - meta:
        name: prod-canary
      url: https://canary
    - meta:
        name: dev-a
      url: https://dev
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, app["backend-names"])
}

func TestComposeTransformListExtractSupportsSourceFromState(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-backends.yaml", "2-extract.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends: []
  backend-names: []
`
	firstLayer := `app:
  backends:
    - meta:
        name: prod-a
    - meta:
        name: prod-b
`
	secondLayer := `operators:
  - kind: list_extract
    source:
      from: state
      path: app.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: meta.name
---
app:
  backend-names: []
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-backends.yaml", []byte(firstLayer), 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/2-extract.yaml", []byte(secondLayer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, app["backend-names"])
}

func TestComposeTransformListExtractWritesToArrayItemTargetPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends:
    - names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends[0].names
    list_extract:
      extract_path: name
---
app:
  backends:
    - names: []
`
	source := `inventory:
  backends:
    - name: prod-a
    - name: prod-b
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	backends := app["backends"].([]interface{})
	first := backends[0].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, first["names"])
}

func TestComposeTransformListExtractWritesToSelectorTargetPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends:
    - name: api
      names: []
    - name: worker
      names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends[name=api].names
    list_extract:
      extract_path: name
---
app:
  backends:
    - name: api
      names: []
    - name: worker
      names: []
`
	source := `inventory:
  backends:
    - name: prod-a
    - name: prod-b
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	backends := app["backends"].([]interface{})
	api := backends[0].(map[string]interface{})
	worker := backends[1].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, api["names"])
	require.Equal([]interface{}{}, worker["names"])
}

func TestComposeTransformListExtractWritesToQuotedSelectorTargetPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `groups:
  - name: abc 123
    ports: []
  - name: xyz
    ports: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: groups[name="abc 123"].ports
    list_extract:
      extract_path: port
---
groups:
  - name: abc 123
    ports: []
  - name: xyz
    ports: []
`
	source := `inventory:
  backends:
    - port: p1
    - port: p2
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	groups := got["groups"].([]interface{})
	first := groups[0].(map[string]interface{})
	second := groups[1].(map[string]interface{})
	require.Equal([]interface{}{"p1", "p2"}, first["ports"])
	require.Equal([]interface{}{}, second["ports"])
}

func TestComposeTransformSelectorTargetReturnsErrorWhenNotUnique(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends:
    - name: api
      names: []
    - name: api
      names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends[name=api].names
    list_extract:
      extract_path: name
---
app:
  backends:
    - name: api
      names: []
    - name: api
      names: []
`
	source := `inventory:
  backends:
    - name: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "matched multiple array items")
}

func TestComposeTransformSelectorTargetReturnsErrorWhenNoMatch(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends:
    - name: worker
      names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends[name=api].names
    list_extract:
      extract_path: name
---
app:
  backends:
    - name: worker
      names: []
`
	source := `inventory:
  backends:
    - name: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "matched no array item")
}

func TestComposeTransformListExtractReturnsErrorWhenArrayTargetIndexOutOfRange(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backends:
    - names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backends.1.names
    list_extract:
      extract_path: name
---
app:
  backends:
    - names: []
`
	source := `inventory:
  backends:
    - name: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "index 1 out of range")
}

func TestComposeTransformListExtractReturnsErrorWhenExtractPathMissing(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: meta.name
---
app:
  backend-names: []
`
	source := `inventory:
  backends:
    - meta:
        id: a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "extract_path \"meta.name\" not found")
}

func TestComposeTransformListExtractReturnsErrorWhenExtractPathIsNotString(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: meta.name
---
app:
  backend-names: []
`
	source := `inventory:
  backends:
    - meta:
        name: 123
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "extract_path \"meta.name\" must resolve to string")
}

func TestComposeTransformListExtractReturnsErrorForInvalidIncludeMode(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: meta.name
      include_mode: invalid
---
app:
  backend-names: []
`
	source := `inventory:
  backends:
    - meta:
        name: prod-a
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "list_extract.include_mode")
}

func TestComposeTransformReplaceValuesSupportsStringSource(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  url: http://foo.example
`
	layer := `operators:
  - kind: replace_values
    source:
      from: state
      path: app.url
    target:
      path: app.url
    replace_values:
      old: foo
      new: bar
---
app: {}
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal("http://bar.example", app["url"])
}

func TestComposeTransformReplaceValuesNonRecursiveOnlyReplacesTopLevelValues(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  labels:
    env: foo
    nested:
      note: foo
    arr: [foo, {note: foo}]
    foo-key: keep
`
	layer := `operators:
  - kind: replace_values
    source:
      from: state
      path: app.labels
    target:
      path: app.labels
    replace_values:
      old: foo
      new: bar
      recursive: false
---
app: {}
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	labels := app["labels"].(map[string]interface{})
	require.Equal("bar", labels["env"])
	require.Equal("keep", labels["foo-key"])

	nested := labels["nested"].(map[string]interface{})
	require.Equal("foo", nested["note"])

	arr := labels["arr"].([]interface{})
	require.Equal("foo", arr[0])
	arrItem := arr[1].(map[string]interface{})
	require.Equal("foo", arrItem["note"])
}

func TestComposeTransformReplaceValuesRecursiveReplacesNestedAndArrayValues(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  payload:
    name: foo-name
    ports: [foo-a, foo-b]
    nested:
      text: foo-c
    keep-key: x
`
	layer := `operators:
  - kind: replace_values
    source:
      from: state
      path: app.payload
    target:
      path: app.payload
    replace_values:
      old: foo
      new: bar
      recursive: true
---
app: {}
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	payload := app["payload"].(map[string]interface{})

	require.Equal("bar-name", payload["name"])
	require.Equal([]interface{}{"bar-a", "bar-b"}, payload["ports"])
	nested := payload["nested"].(map[string]interface{})
	require.Equal("bar-c", nested["text"])
	require.Equal("x", payload["keep-key"])
}

func TestComposeTransformReplaceValuesReturnsErrorWhenOldEmpty(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  url: http://foo.example
`
	layer := `operators:
  - kind: replace_values
    source:
      from: state
      path: app.url
    target:
      path: app.url
    replace_values:
      old: ""
      new: bar
---
app: {}
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "replace_values.old")
}

func TestComposeTransformReplaceValuesPrintsOriginalValuesWhenEnabled(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  payload:
    one: foo-a
    two: foo-b
    three: keep
`
	layer := `operators:
  - kind: replace_values
    source:
      from: state
      path: app.payload
    target:
      path: app.payload
    replace_values:
      old: foo
      new: bar
      recursive: false
      print_original: true
---
app: {}
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	var logs bytes.Buffer
	c.SetTransformLogWriter(&logs)

	_, err = c.Run()
	require.NoError(err)
	require.Contains(logs.String(), "foo-a")
	require.Contains(logs.String(), "foo-b")
	require.NotContains(logs.String(), "keep")
}

func TestComposeOutputUsesTwoSpaceIndentForNestedLists(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", nil)
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte("app:\n  db:\n    ports:\n      - 5432\n"), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)
	require.Contains(out, "ports:\n    - 5432\n")
}

func TestComposeOperatorsMetadataSupportsTransformThenMerge(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  backend-names: []
`
	layer := `operators:
  - kind: list_extract
    source:
      from: file
      file: source.yaml
      path: inventory.backends
    target:
      path: app.backend-names
    list_extract:
      extract_path: name
      include: ["prod-"]
  - kind: merge
    source:
      from: layer
---
app:
  backend-names: []
`
	source := `inventory:
  backends:
    - name: prod-a
    - name: dev-a
    - name: prod-b
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)
	err = fs.WriteFile("source.yaml", []byte(source), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal([]interface{}{"prod-a", "prod-b"}, app["backend-names"])
}

func TestComposeOperatorsCanInterleaveMergeAndStateTransforms(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := `app:
  url: http://base.example
`
	layer := `operators:
  - kind: merge
    source:
      from: layer
  - kind: replace_values
    source:
      from: state
      path: app.url
    target:
      path: app.url
    replace_values:
      old: foo
      new: bar
  - kind: merge
    source:
      from: layer
---
app:
  url: http://foo.example
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	out, err := c.Run()
	require.NoError(err)

	var got map[string]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[string]interface{})
	require.Equal("http://bar.example", app["url"])
}

func TestComposeReturnsErrorForLegacyMetadataField(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	fs := c.GetFilesystem()

	base := "app: {}\n"
	layer := `transform:
  kind: replace_values
  source:
    from: state
    path: app.url
  target:
    path: app.url
  replace_values:
    old: foo
    new: bar
---
app:
  url: http://foo.example
`

	err := fs.WriteFile("base.yaml", []byte(base), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte(layer), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "legacy metadata field")
}
