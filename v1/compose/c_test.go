package compose_test

import (
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
	layer := `merge:
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
	layer := `merge:
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
	layer := `merge:
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
	layer := `merge:
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
	layer := `merge:
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

func TestComposeExtractLayerPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	c.ExtractLayerPath = "app.db"
	fs := c.GetFilesystem()

	base := `app:
  db:
    host: base
    pool: 10
    ports: [5432]
  cache: true
keep: value
`
	layer := `noise: should-not-merge
app:
  db:
    host: layer
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
	db := app["db"].(map[string]interface{})
	require.Equal("layer", db["host"])
	require.Equal(10, db["pool"])
	require.Equal([]interface{}{5433}, db["ports"])
	require.Equal(true, app["cache"])
	require.Equal("value", got["keep"])
	require.NotContains(got, "noise")
}

func TestComposeExtractLayerPathSkipsLayerWhenPathMissing(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	c.ExtractLayerPath = "app.db"
	fs := c.GetFilesystem()

	base := `app:
  db:
    host: base
  cache: true
`
	layer := `app:
  cache: false
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
	require.Equal("base", db["host"])
	require.Equal(true, app["cache"])
}

func TestComposeExtractLayerPathSupportsEscapedDotKey(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	c.ExtractLayerPath = `app.db\.main`
	fs := c.GetFilesystem()

	base := `app:
  db.main:
    ports: [5432]
`
	layer := `app:
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
	require.Equal([]interface{}{5433}, dbMain["ports"])
}

func TestComposeReturnsErrorForInvalidExtractLayerPath(t *testing.T) {
	require := require.New(t)

	c := compose.NewMock("base.yaml", []string{"1-layer.yaml"})
	c.ExtractLayerPath = "app."
	fs := c.GetFilesystem()

	err := fs.WriteFile("base.yaml", []byte(baseYAML), 0755)
	require.NoError(err)
	err = fs.Mkdir("base.yaml.d", 0755)
	require.NoError(err)
	err = fs.WriteFile("base.yaml.d/1-layer.yaml", []byte("xmas: false\n"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "invalid extract layer path")
}
