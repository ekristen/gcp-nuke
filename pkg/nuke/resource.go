package nuke

import (
	"google.golang.org/api/option"

	"github.com/ekristen/libnuke/pkg/registry"
)

const (
	Workspace    registry.Scope = "workspace"
	Organization registry.Scope = "organization"
	Project      registry.Scope = "project"
)

type ListerOpts struct {
	Project       *string
	Region        *string
	Zones         []string
	ClientOptions []option.ClientOption
}
