package compose_test

import (
	"path"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

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

	var got map[interface{}]interface{}
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

	var got map[interface{}]interface{}
	err = yaml.Unmarshal([]byte(out), &got)
	require.NoError(err)

	app := got["app"].(map[interface{}]interface{})
	db := app["db"].(map[interface{}]interface{})
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
