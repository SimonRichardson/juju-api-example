package api

import (
	"github.com/SimonRichardson/juju-api-example/client"

	"github.com/juju/errors"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/modelmanager"
)

type ModelsAPI struct {
	client *client.Client
}

func NewModelsAPI(client *client.Client) *ModelsAPI {
	return &ModelsAPI{
		client: client,
	}
}

func (s *ModelsAPI) Models() ([]base.UserModel, error) {
	accountDetails, err := s.client.AccountDetails()
	if err != nil {
		return nil, errors.Trace(err)
	}

	root, err := s.client.NewAPIRoot()
	if err != nil {
		return nil, errors.Trace(err)
	}

	modelAPI := modelmanager.NewClient(root)
	defer modelAPI.Close()

	return modelAPI.ListModels(accountDetails.User)
}
