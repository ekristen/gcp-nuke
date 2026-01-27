package resources

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	certificatemanager "cloud.google.com/go/certificatemanager/apiv1"
	"cloud.google.com/go/certificatemanager/apiv1/certificatemanagerpb"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const CertificateManagerCertificateResource = "CertificateManagerCertificate"

func init() {
	registry.Register(&registry.Registration{
		Name:     CertificateManagerCertificateResource,
		Scope:    nuke.Project,
		Resource: &CertificateManagerCertificate{},
		Lister:   &CertificateManagerCertificateLister{},
	})
}

type CertificateManagerCertificateLister struct {
	svc *certificatemanager.Client
}

func (l *CertificateManagerCertificateLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if l.svc == nil {
		var err error
		l.svc, err = certificatemanager.NewClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	if err := opts.BeforeList(nuke.Global, "certificatemanager.googleapis.com", CertificateManagerCertificateResource); err == nil {
		globalResources, err := l.listLocation(ctx, opts, "global")
		if err != nil {
			logrus.WithError(err).Error("unable to list global certificate manager certificates")
		} else {
			resources = append(resources, globalResources...)
		}
	}

	if err := opts.BeforeList(nuke.Regional, "certificatemanager.googleapis.com", CertificateManagerCertificateResource); err == nil {
		regionalResources, err := l.listLocation(ctx, opts, *opts.Region)
		if err != nil {
			logrus.WithError(err).Error("unable to list regional certificate manager certificates")
		} else {
			resources = append(resources, regionalResources...)
		}
	}

	return resources, nil
}

func (l *CertificateManagerCertificateLister) listLocation(ctx context.Context, opts *nuke.ListerOpts, location string) ([]resource.Resource, error) {
	var resources []resource.Resource

	req := &certificatemanagerpb.ListCertificatesRequest{
		Parent: fmt.Sprintf("projects/%s/locations/%s", *opts.Project, location),
	}
	it := l.svc.ListCertificates(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate certificate manager certificates")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CertificateManagerCertificate{
			svc:      l.svc,
			project:  opts.Project,
			Location: &location,
			Name:     &name,
			FullName: &resp.Name,
			Labels:   resp.Labels,
		})
	}

	return resources, nil
}

func (l *CertificateManagerCertificateLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type CertificateManagerCertificate struct {
	svc      *certificatemanager.Client
	removeOp *certificatemanager.DeleteCertificateOperation
	project  *string
	Location *string
	Name     *string
	FullName *string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *CertificateManagerCertificate) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCertificate(ctx, &certificatemanagerpb.DeleteCertificateRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *CertificateManagerCertificate) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CertificateManagerCertificate) String() string {
	return *r.Name
}

func (r *CertificateManagerCertificate) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
