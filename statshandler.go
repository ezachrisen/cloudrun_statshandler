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
func NewTracer(h stats.Handler, revisionName, locationName string) stats.Handler {
	return &traceHandler{
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

// traceHandler wrapper
type traceHandler struct {
	h            stats.Handler
	revisionName string
	locationName string
}

func AddTagKeysToViews(views []*view.View) {

	for i := range views {
		views[i].TagKeys = append(views[i].TagKeys, KeyRevisionName, KeyLocationName)
	}
}

func (th *traceHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {

	ctx = th.addCloudTraceHeader(ctx)
	ctx = th.addMetricTags(ctx)

	return th.h.TagRPC(ctx, ti)
}

func (th *traceHandler) addMetricTags(ctx context.Context) context.Context {

	log.WithContext(ctx).Infof("IN TAGRPC %v", *tag.FromContext(ctx))

	ctx, err := tag.New(ctx, tag.Upsert(KeyRevisionName, th.revisionName))
	if err != nil {
		log.WithContext(ctx).Warnf("addMetricTags: Error adding tags: %v", err)
	}

	ctx, err = tag.New(ctx, tag.Upsert(KeyLocationName, th.locationName))
	if err != nil {
		log.WithContext(ctx).Warnf("addMetricTags: Error adding tags: %v", err)
	}

	log.WithContext(ctx).Infof("after setting TAGS %v\n", *tag.FromContext(ctx))
	return ctx

}

func (th *traceHandler) addCloudTraceHeader(ctx context.Context) context.Context {

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

func (th *traceHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	th.h.HandleRPC(ctx, s)
}

func (th *traceHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return th.h.TagConn(ctx, cti)
}

func (th *traceHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	th.h.HandleConn(ctx, cs)
}
