package list

import (
	"fmt"
	"sort"

	"github.com/fatih/color"
	"github.com/urfave/cli/v2"

	"github.com/ekristen/libnuke/pkg/registry"

	"github.com/ekristen/gcp-nuke/pkg/commands/global"
	"github.com/ekristen/gcp-nuke/pkg/common"
	"github.com/ekristen/gcp-nuke/pkg/nuke"

	_ "github.com/ekristen/gcp-nuke/resources"
)

func execute(c *cli.Context) error {
	ls := registry.GetNames()

	sort.Strings(ls)

	for _, name := range ls {
		reg := registry.GetRegistration(name)

		if reg.AlternativeResource != "" {
			_, _ = color.New(color.Bold).Printf("%-55s\n", name)
			_, _ = color.New(color.Bold, color.FgYellow).Printf("  > %-55s", reg.AlternativeResource)
			_, _ = color.New(color.FgCyan).Printf("alternative resource\n")
		} else {
			_, _ = color.New(color.Bold).Printf("%-55s", name)
			c := color.FgGreen
			if reg.Scope == nuke.Organization {
				c = color.FgHiGreen
			} else if reg.Scope == nuke.Project {
				c = color.FgHiBlue
			}
			_, _ = color.New(c).Printf(fmt.Sprintf("%s\n", string(reg.Scope))) // nolint: govet
		}
	}

	return nil
}

func init() {
	cmd := &cli.Command{
		Name:    "resource-types",
		Aliases: []string{"list-resources"},
		Usage:   "list available resources to nuke",
		Flags:   global.Flags(),
		Before:  global.Before,
		Action:  execute,
	}

	common.RegisterCommand(cmd)
}
