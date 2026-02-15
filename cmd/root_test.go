package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRootCmd(t *testing.T) {
	require := require.New(t)
	rootCmd.SetArgs([]string{"a.yaml"})
	err := rootCmd.Execute()
	require.NoError(err)
}
