package fsutil_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fsutil"
)

func TestFileExists(t *testing.T) {
	t.Parallel()

	td := fs.NewDir(t, "enduro-workflows-test",
		fs.WithDir("a", fs.WithFile("needle", "")),
	)

	assert.Assert(t, fsutil.FileExists(td.Join("a", "needle")))
	assert.Assert(t, !fsutil.FileExists(td.Join("b")))
}

func TestFindFilename(t *testing.T) {
	t.Parallel()

	td := fs.NewDir(t, "enduro-workflows-test",
		fs.WithDir("a",
			fs.WithFile("needle", ""),
			fs.WithFile("hay", ""),
		),
		fs.WithDir("needle"),
	)

	t.Run("Find files", func(t *testing.T) {
		t.Parallel()

		got, err := fsutil.FindFilename(td.Path(), "needle")
		assert.NilError(t, err)
		assert.DeepEqual(t, got, []string{
			td.Join("a", "needle"),
			td.Join("needle"),
		})
	})

	t.Run("No filename matches", func(t *testing.T) {
		t.Parallel()

		got, err := fsutil.FindFilename(td.Path(), "cat")
		assert.NilError(t, err)
		assert.DeepEqual(t, got, []string(nil))
	})
}
