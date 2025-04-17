package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const VPCNATResource = "VPCNAT"

func init() {
	registry.Register(&registry.Registration{
		Name:     VPCNATResource,
		Scope:    nuke.Project,
		Resource: &VPCNAT{},
		Lister:   &VPCNATLister{},
	})
}

type VPCNATLister struct {
	svc *compute.RoutersClient
}

func (l *VPCNATLister) Close() {
	if l.svc != nil {
		l.svc.Close()
	}
}

func (l *VPCNATLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRoutersRESTClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	// List all routers in the region
	req := &computepb.ListRoutersRequest{
		Project: *opts.Project,
		Region:  *opts.Region,
	}

	it := l.svc.List(ctx, req)
	for {
		router, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate routers")
			break
		}

		// Check for NAT configurations
		if router.Nats != nil && len(router.Nats) > 0 {
			for _, nat := range router.Nats {
				logrus.WithFields(logrus.Fields{
					"nat":    *nat.Name,
					"router": *router.Name,
					"region": *opts.Region,
				}).Debug("Found NAT configuration")

				resources = append(resources, &VPCNAT{
					svc:        l.svc,
					project:    opts.Project,
					region:     opts.Region,
					routerName: router.Name,
					Name:       nat.Name,
				})
			}
		}
	}

	return resources, nil
}

type VPCNAT struct {
	svc        *compute.RoutersClient
	removeOp   *compute.Operation
	project    *string
	region     *string
	routerName *string
	Name       *string
}

// This method checks if the router exists and if it has our NAT
func (r *VPCNAT) verifyRouterAndNAT(ctx context.Context) (exists bool, hasNAT bool, err error) {
	getReq := &computepb.GetRouterRequest{
		Project: *r.project,
		Region:  *r.region,
		Router:  *r.routerName,
	}

	router, err := r.svc.Get(ctx, getReq)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return false, false, nil // Router doesn't exist
		}
		return false, false, err
	}

	// Router exists, check for NAT
	for _, nat := range router.Nats {
		if nat.Name != nil && *nat.Name == *r.Name {
			return true, true, nil // Router exists and has our NAT
		}
	}

	return true, false, nil // Router exists but doesn't have our NAT
}

func (r *VPCNAT) Remove(ctx context.Context) error {
	// Check if the router exists and if it has our NAT
	routerExists, hasNAT, err := r.verifyRouterAndNAT(ctx)
	if err != nil {
		return fmt.Errorf("error checking router and NAT: %v", err)
	}

	// If router doesn't exist or doesn't have our NAT, consider it removed
	if !routerExists {
		logrus.WithField("router", *r.routerName).Info("Router not found, NAT is considered removed")
		return nil
	}

	if !hasNAT {
		logrus.WithFields(logrus.Fields{
			"nat":    *r.Name,
			"router": *r.routerName,
		}).Info("NAT configuration not found, considering it removed")
		return nil
	}

	// Get the current router configuration
	getReq := &computepb.GetRouterRequest{
		Project: *r.project,
		Region:  *r.region,
		Router:  *r.routerName,
	}

	router, err := r.svc.Get(ctx, getReq)
	if err != nil {
		// Already checked if router exists above, but double-check
		if strings.Contains(err.Error(), "not found") {
			logrus.WithField("router", *r.routerName).Info("Router disappeared, NAT is considered removed")
			return nil
		}
		return fmt.Errorf("error getting router: %v", err)
	}

	// Create a modified router without the NAT configuration
	var updatedNats []*computepb.RouterNat
	for _, nat := range router.Nats {
		if nat.Name == nil || *nat.Name != *r.Name {
			updatedNats = append(updatedNats, nat)
		}
	}

	// Patch the router to remove the NAT configuration
	patchReq := &computepb.PatchRouterRequest{
		Project: *r.project,
		Region:  *r.region,
		Router:  *r.routerName,
		RouterResource: &computepb.Router{
			Nats: updatedNats,
		},
	}

	// Log what we're about to do
	logrus.WithFields(logrus.Fields{
		"nat":    *r.Name,
		"router": *r.routerName,
		"region": *r.region,
	}).Info("Removing NAT configuration from router")

	r.removeOp, err = r.svc.Patch(ctx, patchReq)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logrus.WithField("router", *r.routerName).Info("Router disappeared during patch, NAT is considered removed")
			return nil
		}
		return fmt.Errorf("error patching router: %v", err)
	}

	return nil
}

func (r *VPCNAT) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *VPCNAT) String() string {
	return fmt.Sprintf("%s (on router %s)", *r.Name, *r.routerName)
}

func (r *VPCNAT) HandleWait(ctx context.Context) error {
	// Quick check - if the router doesn't exist or doesn't have our NAT, we're done
	routerExists, hasNAT, err := r.verifyRouterAndNAT(ctx)
	if err == nil && (!routerExists || !hasNAT) {
		logrus.WithFields(logrus.Fields{
			"router_exists": routerExists,
			"has_nat":       hasNAT,
			"router":        *r.routerName,
			"nat":           *r.Name,
		}).Info("Router or NAT no longer exists, considering removed")
		return nil
	}

	if r.removeOp == nil {
		// If we get here without an operation but the NAT still exists, something is wrong
		if routerExists && hasNAT {
			return fmt.Errorf("NAT still exists but no removal operation was started")
		}
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		// If the router is gone, consider the NAT removed
		if strings.Contains(err.Error(), "not found") {
			logrus.WithField("router", *r.routerName).Info("Router not found during polling, NAT is considered removed")
			return nil
		}
		logrus.WithError(err).Error("Error polling router operation")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for NAT removal operation to complete")
	}

	if r.removeOp.Done() {
		if r.removeOp.Proto().GetError() != nil {
			removeErr := fmt.Errorf("delete error on '%s': %s", r.removeOp.Proto().GetTargetLink(), r.removeOp.Proto().GetHttpErrorMessage())

			// If the error indicates the router is gone, that's fine
			if strings.Contains(removeErr.Error(), "not found") {
				logrus.WithField("router", *r.routerName).Info("Router removed during operation, NAT is considered removed")
				return nil
			}

			logrus.WithError(removeErr).WithField("status_code", r.removeOp.Proto().GetError()).Error("Unable to remove NAT configuration")
			return removeErr
		}
	}

	// Final check to make sure the NAT is gone
	routerExists, hasNAT, err = r.verifyRouterAndNAT(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil // Router is gone, so NAT is gone
		}
		return err
	}

	if !routerExists || !hasNAT {
		return nil // NAT is gone
	}

	return fmt.Errorf("NAT still exists after removal operation completed successfully")
}
