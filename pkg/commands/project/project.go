package project

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/ekristen/gcp-nuke/pkg/commands/global"
	"github.com/ekristen/gcp-nuke/pkg/common"
	"github.com/ekristen/gcp-nuke/pkg/gcputil"
)

type CredentialsJSON struct {
	Type                           string `json:"type"`
	ProjectID                      string `json:"project_id"`
	PrivateKeyID                   string `json:"private_key_id"`
	ClientEmail                    string `json:"client_email"`
	ClientID                       string `json:"client_id"`
	Audience                       string `json:"audience"`
	SubjectTokenType               string `json:"subject_token_type"`
	TokenURL                       string `json:"token_url"`
	ServiceAccountImpersonationURL string `json:"service_account_impersonation_url"`
	CredentialSource               struct {
		File   string `json:"file"`
		Format struct {
			Type string `json:"type"`
		} `json:"format"`
	} `json:"credential_source"`
}

func execute(c *cli.Context) error {
	project, err := gcputil.New(c.Context, c.String("project-id"), c.String("impersonate-service-account"))
	if err != nil {
		return err
	}

	fmt.Println("Details")
	fmt.Println("--------------------------------------------------")
	fmt.Println("   Project ID:", project.ID())
	fmt.Printf(" Enabled APIs: %d\n", len(project.GetEnabledAPIs()))
	fmt.Printf("      Regions: %d\n", len(project.Regions))

	creds, err := project.GetCredentials(c.Context)
	if err != nil {
		return err
	}

	var parsed CredentialsJSON
	if err := json.Unmarshal(creds.JSON, &parsed); err != nil {
		return err
	}

	fmt.Println("")
	fmt.Println("Authentication:")
	fmt.Println("--------------------------------------------------")
	fmt.Println(">            Type:", parsed.Type)

	if parsed.Type == "service_account" {
		fmt.Println("> Client Email:", parsed.ClientEmail)
		fmt.Println("> Client ID:", parsed.ClientID)
		fmt.Println("> Private Key ID:", parsed.PrivateKeyID)
	} else if parsed.Type == "external_account" {
		fmt.Println(">        Audience:", parsed.Audience)
		fmt.Println("> Service Account:",
			strings.Replace(
				parsed.ServiceAccountImpersonationURL,
				"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/", "", -1))
		fmt.Println(">     Source.File:", parsed.CredentialSource.File)
		fmt.Println(">   Source.Format:", parsed.CredentialSource.Format.Type)
		if c.String("impersonate-service-account") != "" {
			fmt.Println(">   Impersonating:", c.String("impersonate-service-account"))
		}
	}

	if c.Bool("with-regions") {
		fmt.Println("")
		fmt.Println("Regions:")
		fmt.Println("--------------------------------------------------")
		for _, region := range project.Regions {
			fmt.Println("-", region)
		}
	} else {
		fmt.Println("")
		fmt.Println("Regions: use --with-regions to include regions in the output")
	}

	if c.Bool("with-apis") {
		fmt.Println("")
		fmt.Println("Enabled APIs:")
		fmt.Println("--------------------------------------------------")
		fmt.Println("**Note:** any resource that matches an API that is not enabled will be automatically skipped")
		fmt.Println("")
		for _, api := range project.GetEnabledAPIs() {
			fmt.Println("-", api)
		}
	} else {
		fmt.Println("")
		fmt.Println("Enabled APIs: use --with-apis to include enabled APIs in the output")
	}

	return nil
}

func init() {
	flags := []cli.Flag{
		&cli.PathFlag{
			Name:    "config",
			Aliases: []string{"c"},
			Usage:   "path to config file",
			Value:   "config.yaml",
		},
		&cli.StringFlag{
			Name:     "project-id",
			Usage:    "which GCP project should be nuked",
			EnvVars:  []string{"GCP_NUKE_PROJECT_ID"},
			Required: true,
		},
		&cli.StringFlag{
			Name:    "impersonate-service-account",
			Usage:   "impersonate a service account for all API calls",
			EnvVars: []string{"GCP_NUKE_IMPERSONATE_SERVICE_ACCOUNT"},
		},
		&cli.BoolFlag{
			Name:  "with-regions",
			Usage: "include regions in the output",
		},
		&cli.BoolFlag{
			Name:  "with-apis",
			Usage: "include enabled APIs in the output",
		},
	}

	cmd := &cli.Command{
		Name:        "explain-project",
		Usage:       "explain the project and authentication method used to authenticate against GCP",
		Description: `explain the project and authentication method used to authenticate against GCP`,
		Flags:       append(flags, global.Flags()...),
		Before:      global.Before,
		Action:      execute,
	}

	common.RegisterCommand(cmd)
}
