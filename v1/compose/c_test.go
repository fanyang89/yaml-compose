package compose_test

import (
	"fmt"
	"path"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fanyang89/yaml-compose/v1/compose"
)

func TestLayerSort(t *testing.T) {
	assert := assert.New(t)

	layers := []string{
		"20-a.yaml",
		"10-b.yaml",
		"1-c.yaml",
		"2-d.yaml",
		"3-e.yaml",
	}

	sort.SliceStable(layers, compose.NewLayerComparator(layers))

	assert.Equal(layers, []string{
		"1-c.yaml",
		"2-d.yaml",
		"3-e.yaml",
		"10-b.yaml",
		"20-a.yaml",
	})
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
	fmt.Printf("%s\n", r)
}
