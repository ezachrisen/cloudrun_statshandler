package cloudrun_statshandler

import (
	"context"
	"fmt"

	"net/http"

	httpprop "contrib.go.opencensus.io/exporter/stackdriver/propagation"
	log "github.com/sirupsen/logrus"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

// NewTracer returns a gRPC stats handler that checks
// for cloudTraceHeader in the metadata of the grpc call,
// parses it, and passes it as binary trace to grpc.
// This is a workaround until OpenCensus "understands" GCP trace headers.
func NewTracer(h stats.Handler) stats.Handler {
	return &traceHandler{h}
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
	h stats.Handler
	// revisionName string
	// locationName string
}

func (th *traceHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {
	log.WithContext(ctx).Info("IN TAGRPC")
	ctx, _ = tag.New(ctx, tag.Upsert(KeyRevisionName, "FROM_THE_STATS_HANDLER1"))
	fmt.Printf("TAGS %v\n", *tag.FromContext(ctx))
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get(cloudTraceHeader)) == 0 || len(md.Get(binHeader)) > 0 {
		return th.h.TagRPC(ctx, ti)
	}
	frmt := &httpprop.HTTPFormat{}
	httpReq, _ := http.NewRequest("GET", "/", nil)
	httpReq.Header.Add(cloudTraceHeader, md.Get(cloudTraceHeader)[0])
	sp, ok := frmt.SpanContextFromRequest(httpReq)
	if !ok {
		return th.h.TagRPC(ctx, ti)
	}
	bin := propagation.Binary(sp)
	md = md.Copy()
	md.Set(binHeader, string(bin))
	ctx = metadata.NewIncomingContext(ctx, md)

	return th.h.TagRPC(ctx, ti)
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
