package workflows

import "github.com/artefactual-sdps/enduro/pkg/childwf"

const defaultAPISUsername = "sfa-enduro"

func apisUsername(user *childwf.User) string {
	if user == nil {
		return defaultAPISUsername
	}

	return user.Email
}
