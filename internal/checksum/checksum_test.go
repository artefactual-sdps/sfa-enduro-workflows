package checksum_test

import (
	"testing"

	"gotest.tools/v3/assert"
	"gotest.tools/v3/fs"

	"github.com/artefactual-sdps/sfa-enduro-workflows/internal/checksum"
)

func TestGenerateHash(t *testing.T) {
	t.Parallel()

	td := fs.NewDir(t, "sfa-enduro-workflows-test",
		fs.WithFile("file.txt", "abc"),
		fs.WithFile("empty.txt", ""),
		fs.WithDir("directory"),
	)

	tests := []struct {
		name    string
		path    string
		alg     string
		want    string
		wantErr string
	}{
		{
			name: "Returns an MD5 hash",
			path: td.Join("file.txt"),
			alg:  "MD5",
			want: "900150983cd24fb0d6963f7d28e17f72",
		},
		{
			name: "Returns a SHA-1 hash",
			path: td.Join("file.txt"),
			alg:  "SHA-1",
			want: "a9993e364706816aba3e25717850c26c9cd0d89d",
		},
		{
			name: "Returns a SHA-256 hash",
			path: td.Join("file.txt"),
			alg:  "SHA-256",
			want: "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "Returns a SHA-512 hash",
			path: td.Join("file.txt"),
			alg:  "SHA-512",
			want: "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a" +
				"2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		},
		{
			name: "Returns a hash for an empty file",
			path: td.Join("empty.txt"),
			alg:  "SHA-256",
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "Errors on an unsupported algorithm",
			path:    td.Join("file.txt"),
			alg:     "SHA-224",
			wantErr: `hash algorithm "SHA-224" is not supported`,
		},
		{
			name:    "Errors on a lowercase algorithm",
			path:    td.Join("file.txt"),
			alg:     "md5",
			wantErr: `hash algorithm "md5" is not supported`,
		},
		{
			name:    "Errors on an empty algorithm",
			path:    td.Join("file.txt"),
			wantErr: `hash algorithm "" is not supported`,
		},
		{
			name:    "Errors when opening a missing file",
			path:    td.Join("missing.txt"),
			alg:     "MD5",
			wantErr: "open file: open " + td.Join("missing.txt"),
		},
		{
			name:    "Errors when reading a directory",
			path:    td.Join("directory"),
			alg:     "MD5",
			wantErr: "copy contents:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := checksum.GenerateHash(tt.path, tt.alg)
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Equal(t, got, "")
				return
			}
			assert.NilError(t, err)
			assert.Equal(t, got, tt.want)
		})
	}
}
