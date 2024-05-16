package run

import (
	"context"
	"fmt"
	"github.com/ekristen/gcp-nuke/pkg/gcputil"
	"github.com/ekristen/gcp-nuke/pkg/nuke"
	"github.com/ekristen/libnuke/pkg/scanner"
	"github.com/ekristen/libnuke/pkg/types"
	"github.com/gotidy/ptr"
	"slices"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"

	"github.com/ekristen/gcp-nuke/pkg/commands/global"
	"github.com/ekristen/gcp-nuke/pkg/common"
	libconfig "github.com/ekristen/libnuke/pkg/config"
	libnuke "github.com/ekristen/libnuke/pkg/nuke"
	"github.com/ekristen/libnuke/pkg/registry"
)

func execute(c *cli.Context) error {
	ctx, cancel := context.WithCancel(c.Context)
	defer cancel()

	gcp, err := gcputil.New(ctx, c.String("project-id"))
	if err != nil {
		return err
	}

	//if !gcp.HasOrganizations() {
	//	return fmt.Errorf("no organizations found")
	//}

	if !gcp.HasProjects() {
		return fmt.Errorf("no projects found")
	}

	logrus.Trace("preparing to run nuke")

	params := &libnuke.Parameters{
		Force:      c.Bool("force"),
		ForceSleep: c.Int("force-sleep"),
		Quiet:      c.Bool("quiet"),
		NoDryRun:   c.Bool("no-dry-run"),
		Includes:   c.StringSlice("include"),
		Excludes:   c.StringSlice("exclude"),
	}

	parsedConfig, err := libconfig.New(libconfig.Options{
		Path:         c.Path("config"),
		Deprecations: registry.GetDeprecatedResourceTypeMapping(),
	})
	if err != nil {
		return err
	}

	projectID := c.String("project-id")

	projectConfig := parsedConfig.Accounts[projectID]

	filters, err := parsedConfig.Filters(projectID)
	if err != nil {
		return err
	}

	n := libnuke.New(params, filters, parsedConfig.Settings)

	n.SetRunSleep(5 * time.Second)

	projectResourceTypes := types.ResolveResourceTypes(
		registry.GetNamesForScope(nuke.Project),
		[]types.Collection{
			n.Parameters.Includes,
			parsedConfig.ResourceTypes.GetIncludes(),
			projectConfig.ResourceTypes.GetIncludes(),
		},
		[]types.Collection{
			n.Parameters.Excludes,
			parsedConfig.ResourceTypes.Excludes,
			projectConfig.ResourceTypes.Excludes,
		},
		nil,
		nil,
	)

	/*
			tenantConfig := parsedConfig.Accounts[c.String("tenant-id")]
			tenantResourceTypes := types.ResolveResourceTypes(
				registry.GetNamesForScope(nuke.Organization),
				[]types.Collection{
					n.Parameters.Includes,
					parsedConfig.ResourceTypes.GetIncludes(),
					tenantConfig.ResourceTypes.GetIncludes(),
				},
				[]types.Collection{
					n.Parameters.Excludes,
					parsedConfig.ResourceTypes.Excludes,
					tenantConfig.ResourceTypes.Excludes,
				},
				nil,
				nil,
			)

			subResourceTypes := types.ResolveResourceTypes(
				registry.GetNamesForScope(nuke.Project),
				[]types.Collection{
					n.Parameters.Includes,
					parsedConfig.ResourceTypes.GetIncludes(),
					tenantConfig.ResourceTypes.GetIncludes(),
				},
				[]types.Collection{
					n.Parameters.Excludes,
					parsedConfig.ResourceTypes.Excludes,
					tenantConfig.ResourceTypes.Excludes,
				},
				nil,
				nil,
			)

			if slices.Contains(parsedConfig.Regions, "global") {
				if err := n.RegisterScanner(nuke.Organization, scanner.New("tenant/all", tenantResourceTypes, &nuke.ListerOpts{})); err != nil {
					return err
				}
			}

			logrus.Debug("registering scanner for tenant subscription resources")
			for _, subscriptionId := range tenant.SubscriptionIds {
				logrus.Debug("registering scanner for subscription resources")
				if err := n.RegisterScanner(nuke.Subscription, libnuke.NewScanner("tenant/sub", subResourceTypes, &nuke.ListerOpts{
					Authorizers:    tenant.Authorizers,
					TenantId:       tenant.ID,
					SubscriptionId: subscriptionId,
				})); err != nil {
					return err
				}
			}


		for _, region := range parsedConfig.Regions {
			if region == "global" {
				continue
			}

			for _, subscriptionId := range tenant.SubscriptionIds {
				logrus.Debug("registering scanner for subscription resources")

				for i, resourceGroup := range tenant.ResourceGroups[subscriptionId] {
					logrus.Debugf("registering scanner for resource group resources: rg/%s", resourceGroup)
					if err := n.RegisterScanner(nuke.ResourceGroup, libnuke.NewScanner(fmt.Sprintf("%s/rg%d", region, i), rgResourceTypes, &nuke.ListerOpts{
						Authorizers:    tenant.Authorizers,
						TenantId:       tenant.ID,
						SubscriptionId: subscriptionId,
						ResourceGroup:  resourceGroup,
						Location:       region,
					})); err != nil {
						return err
					}
				}
			}
		}
	*/

	// GCP rest clients have to be closed, this ensures that they are closed properly
	defer func() {
		for _, l := range registry.GetListers() {
			lc, ok := l.(registry.ListerWithClose)
			if ok {
				lc.Close()
			}
		}
	}()

	if slices.Contains(parsedConfig.Regions, "all") {
		parsedConfig.Regions = gcp.Regions

		logrus.Info(
			`"all" detected in region list, only enabled regions and "global" will be used, all others ignored`)

		if len(parsedConfig.Regions) > 1 {
			logrus.Warnf(`additional regions defined along with "all", these will be ignored!`)
		}

		logrus.Infof("The following regions are enabled for the account (%d total):", len(parsedConfig.Regions))

		printableRegions := make([]string, 0)
		for i, region := range parsedConfig.Regions {
			printableRegions = append(printableRegions, region)
			if i%6 == 0 { // print 5 regions per line
				logrus.Infof("> %s", strings.Join(printableRegions, ", "))
				printableRegions = make([]string, 0)
			} else if i == len(parsedConfig.Regions)-1 {
				logrus.Infof("> %s", strings.Join(printableRegions, ", "))
			}
		}
	}

	// Register the scanners for each region that is defined in the configuration.
	for _, regionName := range parsedConfig.Regions {
		if err := n.RegisterScanner(nuke.Project, scanner.New(regionName, projectResourceTypes, &nuke.ListerOpts{
			Project: ptr.String(projectID),
			Region:  ptr.String(regionName),
			Zones:   gcp.GetZones(regionName),
		})); err != nil {
			return err
		}
	}

	logrus.Debug("running ...")

	return n.Run(c.Context)
}

func init() {
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:  "config",
			Usage: "path to config file",
			Value: "config.yaml",
		},
		&cli.StringSliceFlag{
			Name:  "include",
			Usage: "only include this specific resource",
		},
		&cli.StringSliceFlag{
			Name:  "exclude",
			Usage: "exclude this specific resource (this overrides everything)",
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "hide filtered messages from display",
		},
		&cli.BoolFlag{
			Name:  "no-dry-run",
			Usage: "actually run the removal of the resources after discovery",
		},
		&cli.BoolFlag{
			Name:    "no-prompt",
			Usage:   "disable prompting for verification to run",
			Aliases: []string{"force"},
		},
		&cli.IntFlag{
			Name:    "prompt-delay",
			Usage:   "seconds to delay after prompt before running (minimum: 3 seconds)",
			Value:   10,
			Aliases: []string{"force-sleep"},
		},
		&cli.StringSliceFlag{
			Name:  "feature-flag",
			Usage: "enable experimental behaviors that may not be fully tested or supported",
		},
		&cli.StringFlag{
			Name:    "credentials-file",
			Usage:   "the credentials file to use for authentication",
			EnvVars: []string{"GOOGLE_CREDENTIALS"},
		},
		&cli.StringFlag{
			Name:     "project-id",
			Usage:    "which GCP project should be nuked",
			Required: true,
		},
	}

	cmd := &cli.Command{
		Name:    "run",
		Aliases: []string{"nuke"},
		Usage:   "run nuke against a GCP project to remove all configured resources",
		Flags:   append(flags, global.Flags()...),
		Before:  global.Before,
		Action:  execute,
	}

	common.RegisterCommand(cmd)
}
