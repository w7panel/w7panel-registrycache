package null

import (
	"gitee.com/we7coreteam/w7-registry-cache/common/service/registry/types"
	"net/http"
)

// NewAuthorizer returns a null authorizer
func NewAuthorizer() types.Authorizer {
	return &authorizer{}
}

type authorizer struct{}

func (a *authorizer) Modify(_ *http.Request) error {
	// do nothing
	return nil
}
