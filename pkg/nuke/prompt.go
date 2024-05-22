package nuke

import (
	"fmt"
	"time"

	libnuke "github.com/ekristen/libnuke/pkg/nuke"
	"github.com/ekristen/libnuke/pkg/utils"

	"github.com/ekristen/gcp-nuke/pkg/gcputil"
)

type Prompt struct {
	Parameters *libnuke.Parameters
	GCP        *gcputil.GCP
}

// Prompt is the actual function called by the libnuke process during it's run
func (p *Prompt) Prompt() error {
	promptDelay := time.Duration(p.Parameters.ForceSleep) * time.Second

	fmt.Printf("Do you really want to nuke the project with "+
		"the ID '%s'?\n", p.GCP.ID())
	if p.Parameters.Force {
		fmt.Printf("Waiting %v before continuing.\n", promptDelay)
		time.Sleep(promptDelay)
	} else {
		fmt.Printf("Do you want to continue? Enter project ID to continue.\n")
		if err := utils.Prompt(p.GCP.ID()); err != nil {
			return err
		}
	}

	return nil
}
