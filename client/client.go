package client

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/httpbakery"
	"github.com/juju/errors"
	"github.com/juju/names/v4"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/authentication"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/modelmanager"
	"github.com/juju/juju/apiserver/params"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/juju"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/juju/pki"
)

type Client struct {
	store jujuclient.ClientStore

	apiContexts map[string]*apiContext

	controllerName string
	modelName      string
}

func NewClient() (*Client, error) {
	store := modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}
	currentController, err := modelcmd.DetermineCurrentController(store)
	if err != nil {
		return nil, errors.Trace(err)
	}
	currentModel, err := store.CurrentModel(currentController)
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &Client{
		store:          store,
		apiContexts:    make(map[string]*apiContext),
		controllerName: currentController,
		modelName:      currentModel,
	}, nil
}

func (c *Client) AccountDetails() (*jujuclient.AccountDetails, error) {
	return c.store.AccountDetails(c.controllerName)
}

func (c *Client) NewAPIRoot() (api.Connection, error) {
	return c.newAPIRoot("")
}

func (c *Client) NewModelAPIRoot(modelName string) (api.Connection, error) {
	if modelName == "" {
		modelName = c.modelName
	}

	_, err := c.store.ModelByName(c.controllerName, modelName)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, errors.Trace(err)
		}
		// The model isn't known locally, so query the models
		// available in the controller, and cache them locally.
		if err := c.refreshModels(); err != nil {
			return nil, errors.Annotate(err, "refreshing models")
		}
	}
	return c.newAPIRoot(modelName)
}

func (c *Client) newAPIRoot(modelName string) (api.Connection, error) {
	accountDetails, err := c.store.AccountDetails(c.controllerName)
	if err != nil && !errors.IsNotFound(err) {
		return nil, errors.Trace(err)
	}
	// If there are no account details or there's no logged-in
	// user or the user is external, then trigger macaroon authentication
	// by using an empty AccountDetails.
	if accountDetails == nil || accountDetails.User == "" {
		accountDetails = &jujuclient.AccountDetails{}
	} else {
		u := names.NewUserTag(accountDetails.User)
		if !u.IsLocal() {
			if len(accountDetails.Macaroons) == 0 {
				accountDetails = &jujuclient.AccountDetails{}
			} else {
				// If the account has macaroon set, use those to login
				// to avoid an unnecessary auth round trip.
				// Used for embedded commands.
				accountDetails = &jujuclient.AccountDetails{
					User:      u.Id(),
					Macaroons: accountDetails.Macaroons,
				}
			}
		}
	}

	param, err := c.newAPIConnectionParams(
		c.store, c.controllerName, modelName, accountDetails,
	)
	if err != nil {
		return nil, errors.Trace(err)
	}

	conn, err := juju.NewAPIConnection(param)
	if modelName != "" && params.ErrCode(err) == params.CodeModelNotFound {
		return nil, c.missingModelError(c.store, c.controllerName, modelName)
	}
	if redirectErr, ok := errors.Cause(err).(*api.RedirectError); ok {
		return nil, c.newModelMigratedError(c.store, modelName, redirectErr)
	}
	if juju.IsNoAddressesError(err) {
		return nil, errors.New("no controller API addresses; is bootstrap still in progress?")
	}
	return conn, errors.Trace(err)
}

func (c *Client) refreshModels() error {
	root, err := c.NewAPIRoot()
	if err != nil {
		return errors.Trace(err)
	}

	modelManager := modelmanager.NewClient(root)
	defer func() { _ = modelManager.Close() }()

	accountDetails, err := c.store.AccountDetails(c.controllerName)
	if err != nil {
		return errors.Trace(err)
	}

	models, err := modelManager.ListModels(accountDetails.User)
	if err != nil {
		return errors.Trace(err)
	}
	if err := c.storeModels(c.controllerName, models); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (c *Client) storeModels(controllerName string, models []base.UserModel) error {
	modelsToStore := make(map[string]jujuclient.ModelDetails, len(models))
	for _, model := range models {
		modelDetails := jujuclient.ModelDetails{ModelUUID: model.UUID, ModelType: model.Type}
		owner := names.NewUserTag(model.Owner)
		modelName := jujuclient.JoinOwnerModelName(owner, model.Name)
		modelsToStore[modelName] = modelDetails
	}
	if err := c.store.SetModels(controllerName, modelsToStore); err != nil {
		return errors.Trace(err)
	}
	return nil
}

// NewAPIConnectionParams returns a juju.NewAPIConnectionParams with the
// given arguments such that a call to juju.NewAPIConnection with the
// result behaves the same as a call to CommandBase.NewAPIRoot with
// the same arguments.
func (c *Client) newAPIConnectionParams(
	store jujuclient.ClientStore,
	controllerName, modelName string,
	accountDetails *jujuclient.AccountDetails,
) (juju.NewAPIConnectionParams, error) {
	bakeryClient, err := c.bakeryClient(store, controllerName)
	if err != nil {
		return juju.NewAPIConnectionParams{}, errors.Trace(err)
	}

	return newAPIConnectionParams(
		store, controllerName, modelName,
		accountDetails,
		false,
		bakeryClient,
		api.Open,
		getPassword,
	)
}

// BakeryClient returns a macaroon bakery client that
// uses the same HTTP client returned by HTTPClient.
func (c *Client) bakeryClient(store jujuclient.CookieStore, controllerName string) (*httpbakery.Client, error) {
	ctx, err := c.getAPIContext(store, controllerName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ctx.NewBakeryClient(), nil
}

// getAPIContext returns an apiContext for the given controller.
// It will return the same context if called twice for the same controller.
// The context will be closed when closeAPIContexts is called.
func (c *Client) getAPIContext(store jujuclient.CookieStore, controllerName string) (*apiContext, error) {
	if ctx := c.apiContexts[controllerName]; ctx != nil {
		return ctx, nil
	}
	if controllerName == "" {
		return nil, errors.New("cannot get API context from empty controller name")
	}
	ctx, err := newAPIContext(c.store, c.controllerName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	c.apiContexts[controllerName] = ctx
	return ctx, nil
}

func (c *Client) missingModelError(store jujuclient.ClientStore, controllerName, modelName string) error {
	return errors.Errorf("model %q has been removed from the controller, run 'juju models' and switch to one of them.", modelName)
}

func (c *Client) newModelMigratedError(store jujuclient.ClientStore, modelName string, redirErr *api.RedirectError) error {
	// Check if this is a known controller
	allEndpoints := network.CollapseToHostPorts(redirErr.Servers).Strings()
	_, existingName, err := store.ControllerByAPIEndpoints(allEndpoints...)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if existingName != "" {
		mErr := fmt.Sprintf(`Model %q has been migrated to controller %q.
To access it run 'juju switch %s:%s'.`, modelName, existingName, existingName, modelName)

		return modelMigratedError(mErr)
	}

	// CACerts are always valid so no error checking is required here.
	fingerprint, _, err := pki.Fingerprint([]byte(redirErr.CACert))
	if err != nil {
		return err
	}

	ctrlAlias := "new-controller"
	if redirErr.ControllerAlias != "" {
		ctrlAlias = redirErr.ControllerAlias
	}

	var loginCmds []string
	for _, endpoint := range allEndpoints {
		loginCmds = append(loginCmds, fmt.Sprintf("  'juju login %s -c %s'", endpoint, ctrlAlias))
	}

	mErr := fmt.Sprintf(`Model %q has been migrated to another controller.
To access it run one of the following commands (you can replace the -c argument with your own preferred controller name):
%s

New controller fingerprint [%s]`, modelName, strings.Join(loginCmds, "\n"), fingerprint)

	return modelMigratedError(mErr)
}

type modelMigratedError string

func (e modelMigratedError) Error() string {
	return string(e)
}

var errNoNameSpecified = errors.New("no name specified")

func newAPIConnectionParams(
	store jujuclient.ClientStore,
	controllerName,
	modelName string,
	accountDetails *jujuclient.AccountDetails,
	embedded bool,
	bakery *httpbakery.Client,
	apiOpen api.OpenFunc,
	getPassword func(string) (string, error),
) (juju.NewAPIConnectionParams, error) {
	if controllerName == "" {
		return juju.NewAPIConnectionParams{}, errors.Trace(errNoNameSpecified)
	}
	var modelUUID string
	if modelName != "" {
		modelDetails, err := store.ModelByName(controllerName, modelName)
		if err != nil {
			return juju.NewAPIConnectionParams{}, errors.Trace(err)
		}
		modelUUID = modelDetails.ModelUUID
	}
	dialOpts := api.DefaultDialOpts()
	dialOpts.BakeryClient = bakery

	// Embedded clients with macaroons cannot discharge.
	if accountDetails != nil && !embedded {
		bakery.InteractionMethods = []httpbakery.Interactor{
			authentication.NewInteractor(accountDetails.User, getPassword),
			httpbakery.WebBrowserInteractor{},
		}
	}

	return juju.NewAPIConnectionParams{
		Store:          store,
		ControllerName: controllerName,
		AccountDetails: accountDetails,
		ModelUUID:      modelUUID,
		DialOpts:       dialOpts,
		OpenAPI:        apiOpen,
	}, nil
}

func getPassword(username string) (string, error) {
	fmt.Fprintf(os.Stderr, "please enter password for %s: ", username)
	defer fmt.Fprintln(os.Stderr)
	return readPassword(os.Stdin)
}

func readPassword(stdin io.Reader) (string, error) {
	if f, ok := stdin.(*os.File); ok && terminal.IsTerminal(int(f.Fd())) {
		password, err := terminal.ReadPassword(int(f.Fd()))
		return string(password), err
	}
	return readLine(stdin)
}

func readLine(stdin io.Reader) (string, error) {
	// Read one byte at a time to avoid reading beyond the delimiter.
	line, err := bufio.NewReader(byteAtATimeReader{stdin}).ReadString('\n')
	if err != nil {
		return "", errors.Trace(err)
	}
	return line[:len(line)-1], nil
}

type byteAtATimeReader struct {
	io.Reader
}

// Read is part of the io.Reader interface.
func (r byteAtATimeReader) Read(out []byte) (int, error) {
	return r.Reader.Read(out[:1])
}
