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

const CertificateManagerCertificateMapResource = "CertificateManagerCertificateMap"

func init() {
	registry.Register(&registry.Registration{
		Name:      CertificateManagerCertificateMapResource,
		Scope:     nuke.Project,
		Resource:  &CertificateManagerCertificateMap{},
		Lister:    &CertificateManagerCertificateMapLister{},
		DependsOn: []string{CertificateManagerCertificateMapEntryResource},
	})
}

type CertificateManagerCertificateMapLister struct {
	svc *certificatemanager.Client
}

func (l *CertificateManagerCertificateMapLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "certificatemanager.googleapis.com"); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = certificatemanager.NewClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	req := &certificatemanagerpb.ListCertificateMapsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/global", *opts.Project),
	}
	it := l.svc.ListCertificateMaps(ctx, req)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate certificate maps")
			break
		}

		nameParts := strings.Split(resp.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &CertificateManagerCertificateMap{
			svc:      l.svc,
			project:  opts.Project,
			Name:     &name,
			FullName: &resp.Name,
			Labels:   resp.Labels,
		})
	}

	return resources, nil
}

func (l *CertificateManagerCertificateMapLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type CertificateManagerCertificateMap struct {
	svc      *certificatemanager.Client
	removeOp *certificatemanager.DeleteCertificateMapOperation
	project  *string
	Name     *string
	FullName *string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *CertificateManagerCertificateMap) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCertificateMap(ctx, &certificatemanagerpb.DeleteCertificateMapRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *CertificateManagerCertificateMap) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CertificateManagerCertificateMap) String() string {
	return *r.Name
}

func (r *CertificateManagerCertificateMap) HandleWait(ctx context.Context) error {
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
