package api

import (
	"fmt"

	"github.com/SimonRichardson/juju-api-example/client"
	"github.com/juju/charm/v8"
	"github.com/juju/errors"
	apicharms "github.com/juju/juju/api/charms"
	"github.com/juju/juju/cmd/juju/application/utils"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/names/v4"
)

type ApplicationsAPI struct {
	client *client.Client
}

func NewApplicationsAPI(client *client.Client) *ApplicationsAPI {
	return &ApplicationsAPI{
		client: client,
	}
}

type DeployArgs struct {
	ApplicationName string
	Channel         charm.Channel
	Revision        int
	Series          string
	Constraints     constraints.Value
}

func (s *ApplicationsAPI) Deploy(modelName string, charmName string, args DeployArgs) error {
	applicationName := charmName
	if args.ApplicationName != "" {
		applicationName = args.ApplicationName
	}
	if err := names.ValidateApplicationName(applicationName); err != nil {
		return errors.Trace(err)
	}

	apiRoot, err := s.client.NewModelAPIRoot(modelName)
	if err != nil {
		return errors.Trace(err)
	}

	charmAPIClient := apicharms.NewClient(apiRoot)
	defaultCharmSchema := charm.CharmHub
	if charmAPIClient.BestAPIVersion() < 3 {
		defaultCharmSchema = charm.CharmStore
	}

	userRequestedURL, err := resolveCharmURL(charmName, defaultCharmSchema)
	if err != nil {
		return errors.Trace(err)
	}
	// To deploy by revision, the revision number must be in the origin for a
	// charmhub charm and in the url for a charmstore charm.
	if charm.CharmHub.Matches(userRequestedURL.Schema) {
		if userRequestedURL.Revision != -1 {
			return errors.Errorf("cannot specify revision in a charm or bundle name. Please use --revision.")
		}
		if args.Revision != -1 && args.Channel.Empty() {
			return errors.Errorf("specifying a revision requires a channel for future upgrades. Please use --channel")
		}
	} else if charm.CharmStore.Matches(userRequestedURL.Schema) {
		if userRequestedURL.Revision != -1 && args.Revision != -1 && userRequestedURL.Revision != args.Revision {
			return errors.Errorf("two different revisions to deploy: specified %d and %d, please choose one.", userRequestedURL.Revision, args.Revision)
		}
		if userRequestedURL.Revision == -1 && args.Revision != -1 {
			userRequestedURL = userRequestedURL.WithRevision(args.Revision)
		}
	}
	platform, err := utils.DeducePlatform(args.Constraints, args.Series, constraints.Value{})
	if err != nil {
		return errors.Trace(err)
	}

	urlForOrigin := userRequestedURL
	if args.Revision != -1 {
		urlForOrigin = urlForOrigin.WithRevision(args.Revision)
	}
	origin, err := utils.DeduceOrigin(urlForOrigin, args.Channel, platform)
	if err != nil {
		return errors.Trace(err)
	}

	fmt.Println(charmAPIClient.BestAPIVersion(), urlForOrigin, origin)

	return nil
}

func resolveCharmURL(path string, defaultSchema charm.Schema) (*charm.URL, error) {
	var err error
	path, err = charm.EnsureSchema(path, defaultSchema)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return charm.ParseURL(path)
}
