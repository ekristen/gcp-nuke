package resources

import (
	"context"
	"errors"
	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	"google.golang.org/api/iterator"

	"cloud.google.com/go/bigquery"

	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const BigQueryDatasetResource = "BigQueryDataset"

func init() {
	registry.Register(&registry.Registration{
		Name:   BigQueryDatasetResource,
		Scope:  nuke.Project,
		Lister: &BigQueryDatasetLister{},
	})
}

type BigQueryDatasetLister struct {
	svc *bigquery.Client
}

func (l *BigQueryDatasetLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	opts := o.(*nuke.ListerOpts)
	var resources []resource.Resource
	if err := opts.BeforeList(nuke.Regional, "bigquery.googleapis.com"); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = bigquery.NewClient(ctx, *opts.Project, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	// NOTE: you might have to modify the code below to actually work, this currently does not
	// inspect the google go sdk instead is a jumping off point
	it := l.svc.Datasets(ctx)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate networks")
			break
		}

		meta, err := resp.Metadata(ctx)
		if err != nil {
			logrus.WithError(err).Error("unable to get dataset metadata")
			continue
		}

		if meta.Location != ptr.ToString(opts.Region) {
			continue
		}

		resources = append(resources, &BigQueryDataset{
			svc:      l.svc,
			project:  opts.Project,
			region:   opts.Region,
			dataset:  resp,
			Name:     ptr.String(meta.Name),
			Location: ptr.String(meta.Location),
			Labels:   meta.Labels,
		})
	}

	return resources, nil
}

type BigQueryDataset struct {
	svc      *bigquery.Client
	project  *string
	region   *string
	dataset  *bigquery.Dataset
	Name     *string
	Location *string
	Labels   map[string]string `property:"tagPrefix=label"`
}

func (r *BigQueryDataset) Remove(ctx context.Context) error {
	if err := r.removeTables(ctx); err != nil {
		return err
	}

	return r.dataset.Delete(ctx)
}

func (r *BigQueryDataset) removeTables(ctx context.Context) error {
	it := r.dataset.Tables(ctx)
	for {
		resp, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate tables")
			break
		}

		if err := resp.Delete(ctx); err != nil {
			logrus.WithError(err).Error("unable to delete table")
		}
	}

	return nil
}

func (r *BigQueryDataset) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *BigQueryDataset) String() string {
	return *r.Name
}
