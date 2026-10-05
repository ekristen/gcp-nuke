package resources

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/gotidy/ptr"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"cloud.google.com/go/container/apiv1/containerpb"

	"github.com/ekristen/gcp-nuke/pkg/nuke"
)

// fakeClusterManager answers ListClusters the way GKE does: clusters per
// location, and InvalidArgument for locations GKE does not serve.
type fakeClusterManager struct {
	containerpb.UnimplementedClusterManagerServer
	clusters map[string][]string
	rejected map[string]bool
}

func (f *fakeClusterManager) ListClusters(
	_ context.Context, req *containerpb.ListClustersRequest,
) (*containerpb.ListClustersResponse, error) {
	location := req.GetParent()[strings.LastIndex(req.GetParent(), "/")+1:]
	if f.rejected[location] {
		return nil, status.Errorf(codes.InvalidArgument, "Location %q does not exist.", location)
	}

	resp := &containerpb.ListClustersResponse{}
	for _, name := range f.clusters[location] {
		resp.Clusters = append(resp.Clusters, &containerpb.Cluster{Name: name})
	}
	return resp, nil
}

func listGKEClusters(t *testing.T, fake *fakeClusterManager, region string, zones []string) ([]string, error) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer()
	containerpb.RegisterClusterManagerServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)

	lister := &GKEClusterLister{}
	t.Cleanup(lister.Close)

	resources, err := lister.List(context.Background(), &nuke.ListerOpts{
		Project:     ptr.String("sandbox-project"),
		Region:      ptr.String(region),
		Zones:       zones,
		EnabledAPIs: []string{"container.googleapis.com"},
		ClientOptions: []option.ClientOption{
			option.WithEndpoint(listener.Addr().String()),
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		},
	})

	var names []string
	for _, r := range resources {
		names = append(names, *r.(*GKECluster).Name)
	}
	return names, err
}

func TestGKEClusterListerSkipsZonesGKERejects(t *testing.T) {
	fake := &fakeClusterManager{
		clusters: map[string][]string{
			"europe-west4":   {"regional-cluster"},
			"europe-west4-a": {"zonal-cluster"},
		},
		rejected: map[string]bool{"europe-west4-ai1a": true},
	}

	names, err := listGKEClusters(t, fake, "europe-west4",
		[]string{"europe-west4-a", "europe-west4-ai1a", "europe-west4-b"})
	if err != nil {
		t.Fatalf("a zone GKE rejects must not fail the region: %v", err)
	}

	want := []string{"regional-cluster", "zonal-cluster"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("listed %v, want %v", names, want)
	}
}

func TestGKEClusterListerFailsWhenRegionIsRejected(t *testing.T) {
	fake := &fakeClusterManager{rejected: map[string]bool{"europe-west4": true}}

	if _, err := listGKEClusters(t, fake, "europe-west4", []string{"europe-west4-a"}); err == nil {
		t.Fatal("a rejected region must still fail the lister")
	}
}
