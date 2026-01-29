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

const CertificateManagerCertificateMapEntryResource = "CertificateManagerCertificateMapEntry"

func init() {
	registry.Register(&registry.Registration{
		Name:     CertificateManagerCertificateMapEntryResource,
		Scope:    nuke.Project,
		Resource: &CertificateManagerCertificateMapEntry{},
		Lister:   &CertificateManagerCertificateMapEntryLister{},
	})
}

type CertificateManagerCertificateMapEntryLister struct {
	svc *certificatemanager.Client
}

func (l *CertificateManagerCertificateMapEntryLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource
	opts := o.(*nuke.ListerOpts)

	if err := opts.BeforeList(nuke.Global, "certificatemanager.googleapis.com", CertificateManagerCertificateMapEntryResource); err != nil {
		return resources, nil
	}

	if l.svc == nil {
		var err error
		l.svc, err = certificatemanager.NewClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	mapsReq := &certificatemanagerpb.ListCertificateMapsRequest{
		Parent: fmt.Sprintf("projects/%s/locations/global", *opts.Project),
	}
	mapsIt := l.svc.ListCertificateMaps(ctx, mapsReq)
	for {
		certMap, err := mapsIt.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate certificate maps")
			break
		}

		entriesReq := &certificatemanagerpb.ListCertificateMapEntriesRequest{
			Parent: certMap.Name,
		}
		entriesIt := l.svc.ListCertificateMapEntries(ctx, entriesReq)
		for {
			entry, err := entriesIt.Next()
			if errors.Is(err, iterator.Done) {
				break
			}
			if err != nil {
				logrus.WithError(err).Error("unable to iterate certificate map entries")
				break
			}

			nameParts := strings.Split(entry.Name, "/")
			name := nameParts[len(nameParts)-1]

			mapNameParts := strings.Split(certMap.Name, "/")
			mapName := mapNameParts[len(mapNameParts)-1]

			hostname := entry.GetHostname()
			resources = append(resources, &CertificateManagerCertificateMapEntry{
				svc:      l.svc,
				project:  opts.Project,
				Name:     &name,
				FullName: &entry.Name,
				MapName:  &mapName,
				Hostname: &hostname,
				Labels:   entry.Labels,
			})
		}
	}

	return resources, nil
}

func (l *CertificateManagerCertificateMapEntryLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

type CertificateManagerCertificateMapEntry struct {
	svc      *certificatemanager.Client
	removeOp *certificatemanager.DeleteCertificateMapEntryOperation
	project  *string
	Name     *string
	FullName *string
	MapName  *string
	Hostname *string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *CertificateManagerCertificateMapEntry) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCertificateMapEntry(ctx, &certificatemanagerpb.DeleteCertificateMapEntryRequest{
		Name: *r.FullName,
	})
	return err
}

func (r *CertificateManagerCertificateMapEntry) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *CertificateManagerCertificateMapEntry) String() string {
	return *r.Name
}

func (r *CertificateManagerCertificateMapEntry) HandleWait(ctx context.Context) error {
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
