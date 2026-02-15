package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMain(t *testing.T) {
	require := require.New(t)

	originalRun := run
	t.Cleanup(func() {
		run = originalRun
	})

	called := false
	run = func() {
		called = true
	}

	main()
	require.True(called)
}
