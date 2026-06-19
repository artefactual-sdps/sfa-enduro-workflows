package workflows

import (
	"testing"

	"github.com/artefactual-sdps/enduro/pkg/childwf"
	"github.com/stretchr/testify/assert"
)

func TestAPISUsername(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name string
		user *childwf.User
		want string
	}{
		{
			name: "Falls back when user is nil",
			want: defaultAPISUsername,
		},
		{
			name: "Uses user email",
			user: &childwf.User{Email: "operator@example.com"},
			want: "operator@example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, apisUsername(tt.user))
		})
	}
}
