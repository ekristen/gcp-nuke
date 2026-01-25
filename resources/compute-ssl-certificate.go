package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const ComputeSSLCertificateResource = "ComputeSSLCertificate"

func init() {
	registry.Register(&registry.Registration{
		Name:     ComputeSSLCertificateResource,
		Scope:    nuke.Project,
		Resource: &ComputeSSLCertificate{},
		Lister:   &ComputeSSLCertificateLister{},
	})
}

type ComputeSSLCertificateLister struct {
	svc       *compute.RegionSslCertificatesClient
	globalSvc *compute.SslCertificatesClient
}

func (l *ComputeSSLCertificateLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
	if l.globalSvc != nil {
		_ = l.globalSvc.Close()
	}
}

func (l *ComputeSSLCertificateLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "compute.googleapis.com"); err == nil {
		globalResources, err := l.listGlobal(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list global ssl certificates")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "compute.googleapis.com"); err == nil {
		regionalResources, err := l.listRegional(ctx, opts)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional ssl certificates")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *ComputeSSLCertificateLister) listGlobal(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.globalSvc == nil {
		var err error
		l.globalSvc, err = compute.NewSslCertificatesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListSslCertificatesRequest{
		Project: *opts.Project,
	}
	it := l.globalSvc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		resources = append(resources, &ComputeSSLCertificate{
			globalSvc: l.globalSvc,
			Name:      resp.Name,
			project:   opts.Project,
		})
	}

	return resources, nil
}

func (l *ComputeSSLCertificateLister) listRegional(ctx context.Context, opts *nuke.ListerOpts) ([]resource.Resource, error) {
	var resources []resource.Resource

	if l.svc == nil {
		var err error
		l.svc, err = compute.NewRegionSslCertificatesRESTClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &computepb.ListRegionSslCertificatesRequest{
		Project: *opts.Project,
		Region:  *opts.Region,
	}
	it := l.svc.List(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		certResource := &ComputeSSLCertificate{
			svc:       l.svc,
			project:   opts.Project,
			region:    opts.Region,
			Name:      resp.Name,
			Type:      resp.Type,
			Domain:    ptr.String(strings.Join(resp.GetSubjectAlternativeNames(), ",")),
			ExpiresAt: resp.ExpireTime,
		}

		resources = append(resources, certResource)
	}

	return resources, nil
}

type ComputeSSLCertificate struct {
	svc       *compute.RegionSslCertificatesClient
	globalSvc *compute.SslCertificatesClient
	removeOp  *compute.Operation
	project   *string
	region    *string
	Name      *string
	Type      *string
	Domain    *string
	ExpiresAt *string
}

func (r *ComputeSSLCertificate) Remove(ctx context.Context) error {
	if r.svc != nil {
		return r.removeRegional(ctx)
	} else if r.globalSvc != nil {
		return r.removeGlobal(ctx)
	}

	return errors.New("unable to determine service")
}

func (r *ComputeSSLCertificate) removeGlobal(ctx context.Context) (err error) {
	r.removeOp, err = r.globalSvc.Delete(ctx, &computepb.DeleteSslCertificateRequest{
		Project:        *r.project,
		SslCertificate: *r.Name,
	})
	return err
}

func (r *ComputeSSLCertificate) removeRegional(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.Delete(ctx, &computepb.DeleteRegionSslCertificateRequest{
		Project:        *r.project,
		Region:         *r.region,
		SslCertificate: *r.Name,
	})
	return err
}

func (r *ComputeSSLCertificate) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *ComputeSSLCertificate) String() string {
	return *r.Name
}
