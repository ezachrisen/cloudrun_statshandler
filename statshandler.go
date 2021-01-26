// cloudrun_statshandler adds functionality to the OpenCensus gRPC plugin (go.opencensus.io/plugin/ocgrpc) for Google Cloud, specifically Cloud Run. It may work with other Google Cloud platforms as well.
//
// We provide a wrapper around stats.Handler (google.golang.org/grpc/stats), which does two things:
// 1. Translates from the Google Cloud Platform trace ID header to the gRPC trace header
// 2. Adds the Cloud Run revision name to the go.opencensus.io/tag map, so that metrics
// reported by the OpenCensus gRPC plugin are tagged with it. That enables us to monitor A/B
// deployments.
package cloudrun_statshandler

import (
	"context"

	"net/http"

	log "github.com/sirupsen/logrus"

	httpprop "contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

const (
	// cloudTraceHeader is the gRPC metadata key where we'll receive the
	// Google Cloud Platform added trace context ID
	cloudTraceHeader = "X-Cloud-Trace-Context"

	// binHeader is the metadata header key where gRPC expects the
	// trace context ID to be.
	// We'll put the value from cloudTraceHeader here.
	binHeader = "grpc-trace-bin"
)

var (
	// Key for the opencensus metric tag. OpenCensus stores the tags in a tag map in the
	// context. The key name must be unique within the tag map.
	KeyRevisionName = tag.MustNewKey("cloud_run_revision_name")
)

// NewHandler returns a wrapper around a gRPC stats handler
// Provide the Cloud Run revision and location names
func NewHandler(h stats.Handler, revisionName string) stats.Handler {
	return &statsHandler{
		h:            h,
		revisionName: revisionName,
	}
}

// statsHandler wrapper
type statsHandler struct {
	h            stats.Handler
	revisionName string
}

// AddTagKeysToViews adds the revision name and location name tags to the the
// list of go.opencensus.io/stats/view passed in.
func AddTagKeysToViews(views []*view.View) {
	for i := range views {
		views[i].TagKeys = append(views[i].TagKeys, KeyRevisionName)
	}
}

// TagRPC is called by Opencensus to apply metadata to the context. It is called for every
// request.
func (th *statsHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {

	ctx = th.addCloudTraceHeader(ctx)
	//! Important: the order matters.
	// If you call addMetricTags before TagRPC, the tags you added are overwritten
	ctx = th.h.TagRPC(ctx, ti)
	ctx = th.addMetricTags(ctx)

	return ctx
}

// HandleRPC satisfies the stats.Handler interface
func (th *statsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	th.h.HandleRPC(ctx, s)
}

// TagConn satisfies the stats.Handler interface
func (th *statsHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return th.h.TagConn(ctx, cti)
}

// HandleConn satisfies the stats.Handler interface
func (th *statsHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	th.h.HandleConn(ctx, cs)
}

// addMetricTags applies the tags to the OpenCensus tag map in the request context.
func (th *statsHandler) addMetricTags(ctx context.Context) context.Context {

	//logTags(ctx)

	ctx, err := tag.New(ctx, tag.Upsert(KeyRevisionName, th.revisionName))
	if err != nil {
		log.WithContext(ctx).Warnf("addMetricTags: Error adding tags: %v", err)
	}

	//logTags(ctx)
	return ctx
}

// For debugging
func logTags(ctx context.Context) {
	m := tag.FromContext(ctx)
	if m != nil {
		log.WithContext(ctx).Infof("TAGS: %v", *m)
	}
}

// addCloudTraceHeader takes the incoming GCP trace header and puts it into the
// gRPC metadata field.
func (th *statsHandler) addCloudTraceHeader(ctx context.Context) context.Context {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get(cloudTraceHeader)) == 0 || len(md.Get(binHeader)) > 0 {
		return ctx
	}

	frmt := &httpprop.HTTPFormat{}
	httpReq, _ := http.NewRequest("GET", "/", nil)
	httpReq.Header.Add(cloudTraceHeader, md.Get(cloudTraceHeader)[0])
	sp, ok := frmt.SpanContextFromRequest(httpReq)
	if !ok {
		return ctx
	}

	bin := propagation.Binary(sp)
	md = md.Copy()
	md.Set(binHeader, string(bin))
	ctx = metadata.NewIncomingContext(ctx, md)
	return ctx
}
