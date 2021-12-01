package api

import (
	"github.com/SimonRichardson/juju-api-example/client"
	"github.com/SimonRichardson/juju-api-example/common"
	"github.com/juju/charm/v8"
	"github.com/juju/clock"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/juju/api/application"
	"github.com/juju/juju/api/base"
	apicharms "github.com/juju/juju/api/charms"
	commoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/api/modelconfig"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/cmd/juju/application/utils"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/series"
	"github.com/juju/juju/environs/config"
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
	NumUnits        int
	Channel         charm.Channel
	Revision        int
	Series          string
	WorkloadSeries  set.Strings
	Constraints     constraints.Value
	ImageStream     string
}

func (s *ApplicationsAPI) Deploy(modelName string, charmName string, args DeployArgs) error {
	if args.ApplicationName == "" {
		args.ApplicationName = charmName
	}
	if err := names.ValidateApplicationName(args.ApplicationName); err != nil {
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

	applicationsAPIClient := application.NewClient(apiRoot)

	modelAPIClient := modelconfig.NewClient(apiRoot)
	attrs, err := modelAPIClient.ModelGet()
	if err != nil {
		return errors.Wrap(err, errors.New("cannot fetch model settings"))
	}

	modelConfig, err := config.New(config.NoDefaults, attrs)
	if err != nil {
		return errors.Trace(err)
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

	modelConstraints, err := GetModelConstraints(apiRoot)
	if err != nil {
		return errors.Trace(err)
	}

	platform, err := utils.DeducePlatform(args.Constraints, args.Series, modelConstraints)
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

	if args.WorkloadSeries == nil {
		imageStream := args.ImageStream
		if imageStream == "" {
			imageStream = modelConfig.ImageStream()
		}

		workloadSeries, err := series.WorkloadSeries(clock.WallClock.Now(), userRequestedURL.Series, imageStream)
		if err != nil {
			return errors.Trace(err)
		}
		args.WorkloadSeries = workloadSeries
	}

	return s.prepareAndDeploy(deployContext{
		CharmAPIClient:       charmAPIClient,
		ApplicationAPIClient: applicationsAPIClient,
		ModelAPIClient:       modelAPIClient,
		ModelConfig:          modelConfig,
	}, userRequestedURL, origin, args)
}

type deployContext struct {
	CharmAPIClient       *apicharms.Client
	ApplicationAPIClient *application.Client
	ModelAPIClient       *modelconfig.Client
	ModelConfig          *config.Config
}

// PrepareAndDeploy finishes preparing to deploy a charm store charm,
// then deploys it.
func (s *ApplicationsAPI) prepareAndDeploy(ctx deployContext, charmURL *charm.URL, origin commoncharm.Origin, requestedArgs DeployArgs) error {
	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	rev := -1
	origin.Revision = &rev
	resolved, err := ctx.CharmAPIClient.ResolveCharms([]apicharms.CharmToResolve{{URL: charmURL, Origin: origin}})
	if charm.IsUnsupportedSeriesError(err) {
		return errors.Errorf("%v. Use --force to deploy the charm anyway.", err)
	} else if err != nil {
		return errors.Trace(err)
	}

	if len(resolved) != 1 {
		return errors.Errorf("expected only one resolution, received %d", len(resolved))
	}
	selected := resolved[0]

	selector := common.SeriesSelector{
		CharmURLSeries:      charmURL.Series,
		SeriesFlag:          requestedArgs.Series,
		SupportedSeries:     selected.SupportedSeries,
		SupportedJujuSeries: requestedArgs.WorkloadSeries,
		Conf:                ctx.ModelConfig,
	}

	series, err := selector.CharmSeries()
	if err != nil {
		return errors.Trace(err)
	}
	if err := validateCharmSeriesWithName(series, charmURL.Name, requestedArgs.WorkloadSeries); err != nil {
		return errors.Trace(err)
	}

	origin = selected.Origin.WithSeries(series)
	charmURL = charmURL.WithRevision(*origin.Revision).WithArchitecture(origin.Architecture)

	resultOrigin, err := ctx.CharmAPIClient.AddCharm(charmURL, origin, false)
	if err != nil {
		return errors.Trace(err)
	}

	deployArgs := application.DeployArgs{
		CharmID: application.CharmID{
			URL:    charmURL,
			Origin: resultOrigin,
		},
		ApplicationName: requestedArgs.ApplicationName,
		Series:          resultOrigin.Series,
		NumUnits:        requestedArgs.NumUnits,
		Cons:            requestedArgs.Constraints,
	}
	return ctx.ApplicationAPIClient.Deploy(deployArgs)
}

func resolveCharmURL(path string, defaultSchema charm.Schema) (*charm.URL, error) {
	var err error
	path, err = charm.EnsureSchema(path, defaultSchema)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return charm.ParseURL(path)
}

func validateCharmSeriesWithName(series, name string, workloadSeries set.Strings) error {
	err := validateCharmSeries(series, workloadSeries)
	return charmValidationError(name, errors.Trace(err))
}

func validateCharmSeries(seriesName string, workloadSeries set.Strings) error {
	var found bool
	for _, name := range workloadSeries.Values() {
		if name == seriesName {
			found = true
			break
		}
	}
	if !found {
		return errors.NotSupportedf("series: %s", seriesName)
	}
	return nil
}

func charmValidationError(name string, err error) error {
	if err != nil {
		if errors.IsNotSupported(err) {
			return errors.Errorf("%v is not available on the following %v", name, err)
		}
		return errors.Trace(err)
	}
	return nil
}

func GetModelConstraints(caller base.APICallCloser) (constraints.Value, error) {
	_, backend := base.NewClientFacade(caller, "Client")
	results := new(params.GetConstraintsResults)
	err := backend.FacadeCall("GetModelConstraints", nil, results)
	return results.Constraints, err
}
