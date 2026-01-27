package resources

import (
	"context"
	"errors"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	dataproc "cloud.google.com/go/dataproc/v2/apiv1"
	"cloud.google.com/go/dataproc/v2/apiv1/dataprocpb"
	"google.golang.org/api/iterator"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const DataprocClusterResource = "DataprocCluster"

func init() {
	registry.Register(&registry.Registration{
		Name:     DataprocClusterResource,
		Scope:    nuke.Project,
		Resource: &DataprocCluster{},
		Lister:   &DataprocClusterLister{},
	})
}

type DataprocClusterLister struct {
	svc *dataproc.ClusterControllerClient
}

func (l *DataprocClusterLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *DataprocClusterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "dataproc.googleapis.com", DataprocClusterResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = dataproc.NewClusterControllerClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &dataprocpb.ListClustersRequest{
		ProjectId: *opts.Project,
		Region:    *opts.Region,
	}

	it := l.svc.ListClusters(ctx, req)
	for {
		cluster, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate dataproc clusters")
			break
		}

		resources = append(resources, &DataprocCluster{
			svc:     l.svc,
			Project: opts.Project,
			Region:  opts.Region,
			Name:    ptr.String(cluster.ClusterName),
			State:   ptr.String(cluster.Status.State.String()),
			Labels:  cluster.Labels,
		})
	}

	return resources, nil
}

type DataprocCluster struct {
	svc      *dataproc.ClusterControllerClient
	removeOp *dataproc.DeleteClusterOperation
	Project  *string
	Region   *string
	Name     *string           `description:"The name of the Dataproc cluster"`
	State    *string           `description:"The current state of the cluster"`
	Labels   map[string]string `property:"tagPrefix=label" description:"Labels associated with the cluster"`
}

func (r *DataprocCluster) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCluster(ctx, &dataprocpb.DeleteClusterRequest{
		ProjectId:   *r.Project,
		Region:      *r.Region,
		ClusterName: *r.Name,
	})
	return err
}

func (r *DataprocCluster) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *DataprocCluster) String() string {
	return *r.Name
}

func (r *DataprocCluster) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("dataproc cluster remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
