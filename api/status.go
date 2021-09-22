package api

import (
	"github.com/juju/errors"

	"github.com/juju/juju/apiserver/params"

	"github.com/SimonRichardson/juju-api-example/client"
)

type StatusAPI struct {
	client *client.Client
}

func NewStatusAPI(client *client.Client) *StatusAPI {
	return &StatusAPI{
		client: client,
	}
}

func (s *StatusAPI) FullStatus(patterns []string) (*params.FullStatus, error) {
	root, err := s.client.NewAPIRoot()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return root.Client().Status(nil)
}
