package resources

import (
	"context"
	"errors"
	"strings"

	"github.com/gotidy/ptr"
	"github.com/sirupsen/logrus"

	alloydb "cloud.google.com/go/alloydb/apiv1"
	"cloud.google.com/go/alloydb/apiv1/alloydbpb"
	"google.golang.org/api/iterator"

	liberror "github.com/ekristen/libnuke/pkg/errors"
	"github.com/ekristen/libnuke/pkg/registry"
	"github.com/ekristen/libnuke/pkg/resource"
	"github.com/ekristen/libnuke/pkg/types"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

const AlloyDBClusterResource = "AlloyDBCluster"

func init() {
	registry.Register(&registry.Registration{
		Name:      AlloyDBClusterResource,
		Scope:     nuke.Project,
		Resource:  &AlloyDBCluster{},
		Lister:    &AlloyDBClusterLister{},
		DependsOn: []string{AlloyDBInstanceResource},
	})
}

type AlloyDBClusterLister struct {
	svc *alloydb.AlloyDBAdminClient
}

func (l *AlloyDBClusterLister) Close() {
	if l.svc != nil {
		_ = l.svc.Close()
	}
}

func (l *AlloyDBClusterLister) List(ctx context.Context, o interface{}) ([]resource.Resource, error) {
	var resources []resource.Resource

	opts := o.(*nuke.ListerOpts)
	if err := opts.BeforeList(nuke.Regional, "alloydb.googleapis.com", AlloyDBClusterResource); err != nil {
		return resources, err
	}

	if l.svc == nil {
		var err error
		l.svc, err = alloydb.NewAlloyDBAdminClient(ctx, opts.ClientOptions...)
		if err != nil {
			return nil, err
		}
	}

	req := &alloydbpb.ListClustersRequest{
		Parent: "projects/" + *opts.Project + "/locations/" + *opts.Region,
	}

	it := l.svc.ListClusters(ctx, req)
	for {
		cluster, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			logrus.WithError(err).Error("unable to iterate alloydb clusters")
			break
		}

		nameParts := strings.Split(cluster.Name, "/")
		name := nameParts[len(nameParts)-1]

		resources = append(resources, &AlloyDBCluster{
			svc:      l.svc,
			Project:  opts.Project,
			Region:   opts.Region,
			FullName: ptr.String(cluster.Name),
			Name:     ptr.String(name),
			State:    ptr.String(cluster.State.String()),
			Labels:   cluster.Labels,
		})
	}

	return resources, nil
}

type AlloyDBCluster struct {
	svc      *alloydb.AlloyDBAdminClient
	removeOp *alloydb.DeleteClusterOperation
	Project  *string
	Region   *string
	FullName *string
	Name     *string           `description:"The name of the AlloyDB cluster"`
	State    *string           `description:"The current state of the cluster"`
	Labels   map[string]string `property:"tagPrefix=label" description:"Labels associated with the cluster"`
}

func (r *AlloyDBCluster) Remove(ctx context.Context) (err error) {
	r.removeOp, err = r.svc.DeleteCluster(ctx, &alloydbpb.DeleteClusterRequest{
		Name:  *r.FullName,
		Force: true,
	})
	return err
}

func (r *AlloyDBCluster) Properties() types.Properties {
	return types.NewPropertiesFromStruct(r)
}

func (r *AlloyDBCluster) String() string {
	return *r.Name
}

func (r *AlloyDBCluster) HandleWait(ctx context.Context) error {
	if r.removeOp == nil {
		return nil
	}

	if err := r.removeOp.Poll(ctx); err != nil {
		logrus.WithError(err).Trace("alloydb cluster remove op polling encountered error")
		return err
	}

	if !r.removeOp.Done() {
		return liberror.ErrWaitResource("waiting for operation to complete")
	}

	return nil
}
