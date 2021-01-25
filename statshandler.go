// cloudrun_statshandler adds functionality to the OpenCensus gRPC plugin (go.opencensus.io/plugin/ocgrpc).
// We provide a wrapper around stats.Handler (google.golang.org/grpc/stats), which does two things:
// 1. Translates from the Google Cloud Platform trace ID header to the gRPC trace header
// 2. Adds the Cloud Run revision and location names to the go.opencensus.io/tag map, so that metrics
//    reported by the OpenCensus gRPC plugin are tagged with those values.
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

// NewTracer returns a gRPC stats handler that checks
// for cloudTraceHeader in the metadata of the grpc call,
// parses it, and passes it as binary trace to grpc.
// This is a workaround until OpenCensus "understands" GCP trace headers.
func NewHandler(h stats.Handler, revisionName, locationName string) stats.Handler {
	return &statsHandler{
		h:            h,
		revisionName: revisionName,
		locationName: locationName,
	}
}

const (
	cloudTraceHeader = "X-Cloud-Trace-Context"
	binHeader        = "grpc-trace-bin"
)

var (
	KeyRevisionName = tag.MustNewKey("cloud_run_revision_name")
	KeyLocationName = tag.MustNewKey("cloud_run_location_name")
)

// statsHandler wrapper
type statsHandler struct {
	h            stats.Handler
	revisionName string
	locationName string
}

func AddTagKeysToViews(views []*view.View) {
	for i := range views {
		views[i].TagKeys = append(views[i].TagKeys, KeyRevisionName, KeyLocationName)
	}
}

func (th *statsHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {

	ctx = th.addCloudTraceHeader(ctx)
	ctx = th.addMetricTags(ctx)

	return th.h.TagRPC(ctx, ti)
}

func (th *statsHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	th.h.HandleRPC(ctx, s)
}

func (th *statsHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return th.h.TagConn(ctx, cti)
}

func (th *statsHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	th.h.HandleConn(ctx, cs)
}

func (th *statsHandler) addMetricTags(ctx context.Context) context.Context {

	m := tag.FromContext(ctx)
	if m != nil {
		log.WithContext(ctx).Infof("IN TAGRPC %v", *m)
	}

	ctx, err := tag.New(ctx, tag.Upsert(KeyRevisionName, th.revisionName))
	if err != nil {
		log.WithContext(ctx).Warnf("addMetricTags: Error adding tags: %v", err)
	}

	ctx, err = tag.New(ctx, tag.Upsert(KeyLocationName, th.locationName))
	if err != nil {
		log.WithContext(ctx).Warnf("addMetricTags: Error adding tags: %v", err)
	}

	m = tag.FromContext(ctx)
	if m != nil {
		log.WithContext(ctx).Infof("AFTER setting tagrpcs %v", *m)
	}
	return ctx

}

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
