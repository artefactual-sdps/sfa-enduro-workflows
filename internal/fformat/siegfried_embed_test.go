package fformat_test

import (
	"testing"

	"gotest.tools/v3/assert"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/fformat"
)

func TestSiegfriedEmbed(t *testing.T) {
	t.Parallel()

	t.Run("Identifies a file", func(t *testing.T) {
		t.Parallel()

		sf := fformat.NewSiegfriedEmbed()

		got, err := sf.Identify("siegfried_embed.go")
		assert.NilError(t, err)
		assert.DeepEqual(t, got, &fformat.FileFormat{
			Namespace:  "pronom",
			ID:         "x-fmt/111",
			CommonName: "Plain Text File",
			MIMEType:   "text/plain",
			Basis:      "text match ASCII",
			Warning:    "match on text only; extension mismatch",
		})
	})

	t.Run("Errors when file not found", func(t *testing.T) {
		t.Parallel()

		sf := fformat.NewSiegfriedEmbed()
		_, err := sf.Identify("foobar.txt")
		assert.Error(t, err, "open foobar.txt: no such file or directory")
	})
}

func BenchmarkSiegfried(b *testing.B) {
	b.Run("SiegfriedEmbed", func(b *testing.B) {
		sf := fformat.NewSiegfriedEmbed()
		b.ResetTimer()
		for range b.N {
			sf.Identify("fformat.go")
		}
	})
}
