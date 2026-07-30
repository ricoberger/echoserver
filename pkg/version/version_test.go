package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfo(t *testing.T) {
	Version = "v0.7.0-29-gcc373f2"
	Revision = "cc373f263575773f1349bbd354e803cc85f9edcd"
	Branch = "main"

	require.NotPanics(t, func() {
		Info()
	})
}

func TestBuildContext(t *testing.T) {
	BuildUser = "root"
	BuildDate = "2021-12-23@09:46:17"

	require.NotPanics(t, func() {
		BuildContext()
	})
}
