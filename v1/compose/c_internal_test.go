package compose

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	require := require.New(t)

	c := New("base.yaml", []string{"1-layer.yaml"})
	require.Equal("base.yaml", c.Base)
	require.Equal([]string{"1-layer.yaml"}, c.Layers)
	require.NotNil(c.GetFilesystem())
}

func TestPart(t *testing.T) {
	require := require.New(t)

	prefix, name := part("1-layer.yaml")
	require.Equal("1", prefix)
	require.Equal("layer.yaml", name)

	prefix, name = part("layer.yaml")
	require.Equal("layer.yaml", prefix)
	require.Equal("", name)
}

func TestValidateLayerName(t *testing.T) {
	require := require.New(t)

	require.NoError(validateLayerName("1-a.yaml"))

	err := validateLayerName("bad.yaml")
	require.Error(err)
	require.Contains(err.Error(), "expected <order>-<name>.yaml")

	err = validateLayerName("-a.yaml")
	require.Error(err)
	require.Contains(err.Error(), "expected <order>-<name>.yaml")

	err = validateLayerName("a-1.yaml")
	require.Error(err)
	require.Contains(err.Error(), "expected numeric order prefix")
}

func TestMergeMapsWithNilBase(t *testing.T) {
	require := require.New(t)

	got := mergeMaps(nil, map[string]interface{}{"service": "layer"})
	require.Equal(map[string]interface{}{"service": "layer"}, got)
}

func TestRunReturnsMarshalError(t *testing.T) {
	require := require.New(t)

	originalMarshal := yamlMarshal
	yamlMarshal = func(in interface{}) ([]byte, error) {
		return nil, errors.New("marshal failed")
	}
	t.Cleanup(func() {
		yamlMarshal = originalMarshal
	})

	c := NewMock("base.yaml", nil)
	fs := c.GetFilesystem()
	err := fs.WriteFile("base.yaml", []byte("service: base\n"), 0755)
	require.NoError(err)

	_, err = c.Run()
	require.Error(err)
	require.Contains(err.Error(), "failed to marshal compose file")
}
