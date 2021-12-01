// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package common

import (
	"github.com/juju/charm/v8"
	"github.com/juju/collections/set"
	"github.com/juju/errors"
	"github.com/juju/juju/version"
)

type ModelConfig interface {
	// DefaultSeries returns the configured default Ubuntu series
	// for the environment, and whether the default series was
	// explicitly configured on the environment.
	DefaultSeries() (string, bool)
}

// seriesSelector is a helper type that determines what series the charm should
// be deployed to.
//
// TODO: This type should really have a Validate method, as the force flag is
// really only valid if the seriesFlag is specified. There is code and tests
// that allow the force flag when series isn't specified, but they should
// really be cleaned up. The `deploy` CLI command has tests to ensure that
// --force is only valid with --series.
type SeriesSelector struct {
	// seriesFlag is the series passed to the --series flag on the command line.
	SeriesFlag string
	// charmURLSeries is the series specified as part of the charm URL, i.e.
	// cs:trusty/ubuntu.
	CharmURLSeries string
	// conf is the configuration for the model we're deploying to.
	Conf ModelConfig
	// supportedSeries is the list of series the charm supports.
	SupportedSeries []string
	// supportedJujuSeries is the list of series that juju supports.
	SupportedJujuSeries set.Strings
	// force indicates the user explicitly wants to deploy to a requested
	// series, regardless of whether the charm says it supports that series.
	Force bool
}

// charmSeries determines what series to use with a charm.
// Order of preference is:
// - user requested with --series or defined by bundle when deploying
// - user requested in charm's url (e.g. juju deploy precise/ubuntu)
// - model default (if it matches supported series)
// - default from charm metadata supported series / series in url
// - default LTS
func (s SeriesSelector) CharmSeries() (selectedSeries string, err error) {
	// TODO(sidecar): handle systems

	// User has requested a series with --series.
	if s.SeriesFlag != "" {
		return s.userRequested(s.SeriesFlag)
	}

	// User specified a series in the charm URL, e.g.
	// juju deploy precise/ubuntu.
	if s.CharmURLSeries != "" {
		return s.userRequested(s.CharmURLSeries)
	}

	// No series explicitly requested by the user.
	// Use model default series, if explicitly set and supported by the charm.
	if defaultSeries, explicit := s.Conf.DefaultSeries(); explicit {
		if _, err := charm.SeriesForCharm(defaultSeries, s.SupportedSeries); err == nil {
			// validate the series we get from the charm
			if err := s.validateSeries(defaultSeries); err != nil {
				return "", err
			}
			return defaultSeries, nil
		}
	}

	// Next fall back to the charm's list of series, filtered to what's supported
	// by Juju. Preserve the order of the supported series from the charm
	// metadata, as the order could be out of order compared to Ubuntu series
	// order (precise, xenial, bionic, trusty, etc).
	var supportedSeries []string
	for _, charmSeries := range s.SupportedSeries {
		if s.SupportedJujuSeries.Contains(charmSeries) {
			supportedSeries = append(supportedSeries, charmSeries)
		}
	}
	defaultSeries, err := charm.SeriesForCharm("", supportedSeries)
	if err == nil {
		return defaultSeries, nil
	}

	// Charm hasn't specified a default (likely due to being a local charm
	// deployed by path).  Last chance, best we can do is default to LTS.

	// At this point, because we have no idea what series the charm supports,
	// *everything* requires --force.
	if !s.Force {
		// We know err is not nil due to above, so return the error
		// returned to us from the charm call.
		return "", err
	}

	latestLTS := version.DefaultSupportedLTS()
	return latestLTS, nil
}

// userRequested checks the series the user has requested, and returns it if it
// is supported, or if they used --force.
func (s SeriesSelector) userRequested(requestedSeries string) (string, error) {
	// TODO(sidecar): handle computed series
	series, err := charm.SeriesForCharm(requestedSeries, s.SupportedSeries)
	if s.Force {
		series = requestedSeries
	} else if err != nil {
		return "", err
	}

	// validate the series we get from the charm
	if err := s.validateSeries(series); err != nil {
		return "", err
	}

	return series, nil
}

func (s SeriesSelector) validateSeries(seriesName string) error {
	// if we're forcing then we don't need the following validation checks.
	if len(s.SupportedJujuSeries) == 0 {
		// programming error
		return errors.Errorf("expected supported juju series to exist")
	}
	if s.Force {
		return nil
	}

	if !s.SupportedJujuSeries.Contains(seriesName) {
		return errors.NotSupportedf("series: %s", seriesName)
	}
	return nil
}
