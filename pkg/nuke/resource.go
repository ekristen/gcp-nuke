package nuke

import (
	"fmt"
	"slices"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"

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
	Project                   *string
	Region                    *string
	Zones                     []string
	EnabledAPIs               []string
	ClientOptions             []option.ClientOption
	DisableDeletionProtection bool
}

func (o *ListerOpts) BeforeList(geo Geography, service string, resourceNames ...string) error {
	log := logrus.WithField("geo", geo).
		WithField("service", service).
		WithField("hook", "true")

	resourceName := ""
	if len(resourceNames) > 0 {
		resourceName = resourceNames[0]
		log = log.WithField("resource", resourceName)
	}

	if geo == Global && *o.Region != "global" {
		log.Trace("before-list: skipping resource, global")
		return liberror.ErrSkipRequest("resource is global")
	} else if geo == Regional && *o.Region == "global" {
		log.Trace("before-list: skipping resource, regional")
		return liberror.ErrSkipRequest("resource is regional")
	}

	if !slices.Contains(o.EnabledAPIs, service) {
		log.Warn("before-list: skipping resource, api not enabled")
		return liberror.ErrSkipRequest(fmt.Sprintf("api '%s' not enabled", service))
	}

	log.Trace("before-list: called")

	return nil
}
