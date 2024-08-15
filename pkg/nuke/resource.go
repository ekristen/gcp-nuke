package nuke

import (
	"fmt"
	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
	"slices"

	"github.com/ekristen/libnuke/pkg/registry"
)

type Geography string

const (
	Workspace    registry.Scope = "workspace"
	Organization registry.Scope = "organization"
	Project      registry.Scope = "project"

	Global   Geography = "global"
	Regional Geography = "regional"
	Zonal    Geography = "zonal"
)

type ListerOpts struct {
	Project       *string
	Region        *string
	Zones         []string
	EnabledAPIs   []string
	ClientOptions []option.ClientOption
}

func (o *ListerOpts) BeforeList(geo Geography, service string) error {
	log := logrus.WithField("geo", geo).
		WithField("service", service).
		WithField("hook", "true")

	if geo == Global && *o.Region != "global" {
		log.Trace("before-list: skipping resource, global")
		return liberror.ErrSkipRequest("resource is global")
	} else if geo == Regional && *o.Region == "global" {
		log.Trace("before-list: skipping resource, regional")
		return liberror.ErrSkipRequest("resource is regional")
	}

	if !slices.Contains(o.EnabledAPIs, service) {
		log.Trace("before-list: skipping resource, api not enabled")
		return liberror.ErrSkipRequest(fmt.Sprintf("api '%s' not enabled", service))
	}

	log.Trace("before-list: called")

	return nil
}
