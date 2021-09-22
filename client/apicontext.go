package client

import (
	"net/http"
	"net/url"
	"os"

	"github.com/go-macaroon-bakery/macaroon-bakery/v3/httpbakery"
	"github.com/juju/errors"
	"github.com/juju/idmclient/v2/ussologin"
	"gopkg.in/juju/environschema.v1/form"

	"github.com/juju/juju/jujuclient"
)

// apiContext holds the context required for making connections to
// APIs used by juju.
type apiContext struct {
	// jar holds the internal version of the cookie jar - it has
	// methods that clients should not use, such as Save.
	jar        *domainCookieJar
	interactor httpbakery.Interactor
}

// newAPIContext returns an API context that will use the given
// context for user interactions when authorizing.
// The returned API context must be closed after use.
//
// If ctx is nil, no command-line authorization
// will be supported.
//
// This function is provided for use by commands that cannot use
// CommandBase. Most clients should use that instead.
func newAPIContext(store jujuclient.CookieStore, controllerName string) (*apiContext, error) {
	jar0, err := store.CookieJar(controllerName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	// The JUJU_USER_DOMAIN environment variable specifies
	// the preferred user domain when discharging third party caveats.
	// We set up a cookie jar that will send it to all sites because
	// we don't know where the third party might be.
	jar := &domainCookieJar{
		CookieJar: jar0,
		domain:    os.Getenv("JUJU_USER_DOMAIN"),
	}

	filler := &form.IOFiller{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	interactor := ussologin.NewInteractor(ussologin.StoreTokenGetter{
		Store: jujuclient.NewTokenStore(),
		TokenGetter: ussologin.FormTokenGetter{
			Filler: filler,
			Name:   "juju",
		},
	})

	return &apiContext{
		jar:        jar,
		interactor: interactor,
	}, nil
}

// CookieJar returns the cookie jar used to make
// HTTP requests.
func (ctx *apiContext) CookieJar() http.CookieJar {
	return ctx.jar
}

// NewBakeryClient returns a new httpbakery.Client, using the API context's
// persistent cookie jar and web page visitor.
func (ctx *apiContext) NewBakeryClient() *httpbakery.Client {
	client := httpbakery.NewClient()
	client.Jar = ctx.jar
	if ctx.interactor != nil {
		client.AddInteractor(ctx.interactor)
	}
	return client
}

// Close closes the API context, saving any cookies to the
// persistent cookie jar.
func (ctx *apiContext) Close() error {
	if err := ctx.jar.Save(); err != nil {
		return errors.Annotatef(err, "cannot save cookie jar")
	}
	return nil
}

const domainCookieName = "domain"

// domainCookieJar implements a variant of CookieJar that
// always includes a domain cookie regardless of the site.
type domainCookieJar struct {
	jujuclient.CookieJar
	// domain holds the value of the domain cookie.
	domain string
}

// Cookies implements http.CookieJar.Cookies by
// adding the domain cookie when the domain is non-empty.
func (j *domainCookieJar) Cookies(u *url.URL) []*http.Cookie {
	cookies := j.CookieJar.Cookies(u)
	if j.domain == "" {
		return cookies
	}
	// Allow the site to override if it wants to.
	for _, c := range cookies {
		if c.Name == domainCookieName {
			return cookies
		}
	}
	return append(cookies, &http.Cookie{
		Name:  domainCookieName,
		Value: j.domain,
	})
}
