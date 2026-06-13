package examplemcp

import (
	"context"

	"github.com/easyp-tech/protoc-gen-mcp/mcpruntime"
	emptypb "google.golang.org/protobuf/types/known/emptypb"

	examplev1 "github.com/easyp-tech/protoc-gen-mcp/internal/testproto/example/v1"
)

// NewServer returns a ready-to-run MCP server backed by generated protobuf tools.
func NewServer() (*mcpruntime.Server, error) {
	server := mcpruntime.NewServer("protoc-gen-mcp-example-server", "v0.0.1")

	if err := examplev1.RegisterExampleAPITools(server, Handler{}); err != nil {
		return nil, err
	}

	return server, nil
}

// Handler implements the generated test tool interface.
type Handler struct{}

// CreateReport returns a deterministic response for integration checks.
func (Handler) CreateReport(
	_ context.Context,
	req *examplev1.CreateReportRequest,
) (*examplev1.CreateReportResponse, error) {
	details := &examplev1.ReportDetails{}
	if req.GetDetails() != nil {
		details.Label = req.GetDetails().GetLabel()
	}

	return &examplev1.CreateReportResponse{
		ReportId:   "report-1",
		TotalCount: 42,
		Status:     examplev1.ReportStatus_REPORT_STATUS_OK,
		Details:    details,
		Warnings:   []string{"none"},
	}, nil
}

// Ping returns an empty acknowledgement payload.
func (Handler) Ping(
	_ context.Context,
	_ *examplev1.PingRequest,
) (*examplev1.PingResponse, error) {
	return &examplev1.PingResponse{
		Ack: &emptypb.Empty{},
	}, nil
}

// DescribeAdvancedShapes echoes advanced protobuf shapes for integration checks.
func (Handler) DescribeAdvancedShapes(
	_ context.Context,
	req *examplev1.DescribeAdvancedShapesRequest,
) (*examplev1.DescribeAdvancedShapesResponse, error) {
	resp := &examplev1.DescribeAdvancedShapesResponse{
		Labels:      req.GetLabels(),
		Quantities:  req.GetQuantities(),
		Toggles:     req.GetToggles(),
		Limits:      req.GetLimits(),
		ObservedAt:  req.GetObservedAt(),
		Ttl:         req.GetTtl(),
		Payload:     req.GetPayload(),
		Items:       req.GetItems(),
		Dynamic:     req.GetDynamic(),
		Note:        req.GetNote(),
		Total:       req.GetTotal(),
		Enabled:     req.GetEnabled(),
		Ratio:       req.GetRatio(),
		Mask:        req.GetMask(),
		Blob:        req.GetBlob(),
		SmallTotal:  req.GetSmallTotal(),
		UintTotal:   req.GetUintTotal(),
		HugeTotal:   req.GetHugeTotal(),
		Weight:      req.GetWeight(),
		RawRatio:    req.RawRatio,
		Tree:        req.GetTree(),
		DetailAny:   req.GetDetailAny(),
		DurationAny: req.GetDurationAny(),
	}

	switch selector := req.Selector.(type) {
	case *examplev1.DescribeAdvancedShapesRequest_CityAlias:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityAlias{CityAlias: selector.CityAlias}
	case *examplev1.DescribeAdvancedShapesRequest_CityId:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityId{CityId: selector.CityId}
	case *examplev1.DescribeAdvancedShapesRequest_CityDetails:
		resp.Selector = &examplev1.DescribeAdvancedShapesResponse_CityDetails{CityDetails: selector.CityDetails}
	}

	return resp, nil
}

// HiddenThing handles the hidden RPC for integration checks.
func (Handler) HiddenThing(
	_ context.Context,
	_ *examplev1.HiddenThingRequest,
) (*examplev1.HiddenThingResponse, error) {
	return &examplev1.HiddenThingResponse{}, nil
}

// DescribeScalarShapes echoes plain protobuf scalar kinds for integration checks.
func (Handler) DescribeScalarShapes(
	_ context.Context,
	req *examplev1.DescribeScalarShapesRequest,
) (*examplev1.DescribeScalarShapesResponse, error) {
	return &examplev1.DescribeScalarShapesResponse{
		BoolFlag:              req.GetBoolFlag(),
		TextValue:             req.GetTextValue(),
		BytesValue:            req.GetBytesValue(),
		Int32Value:            req.GetInt32Value(),
		Sint32Value:           req.GetSint32Value(),
		Sfixed32Value:         req.GetSfixed32Value(),
		Uint32Value:           req.GetUint32Value(),
		Fixed32Value:          req.GetFixed32Value(),
		Int64Value:            req.GetInt64Value(),
		Sint64Value:           req.GetSint64Value(),
		Sfixed64Value:         req.GetSfixed64Value(),
		Uint64Value:           req.GetUint64Value(),
		Fixed64Value:          req.GetFixed64Value(),
		FloatValue:            req.GetFloatValue(),
		DoubleValue:           req.GetDoubleValue(),
		Status:                req.GetStatus(),
		Details:               req.GetDetails(),
		Samples:               req.GetSamples(),
		OptionalBoolFlag:      req.OptionalBoolFlag,
		OptionalTextValue:     req.OptionalTextValue,
		OptionalBytesValue:    req.OptionalBytesValue,
		OptionalInt32Value:    req.OptionalInt32Value,
		OptionalSint32Value:   req.OptionalSint32Value,
		OptionalSfixed32Value: req.OptionalSfixed32Value,
		OptionalUint32Value:   req.OptionalUint32Value,
		OptionalFixed32Value:  req.OptionalFixed32Value,
		OptionalInt64Value:    req.OptionalInt64Value,
		OptionalSint64Value:   req.OptionalSint64Value,
		OptionalSfixed64Value: req.OptionalSfixed64Value,
		OptionalUint64Value:   req.OptionalUint64Value,
		OptionalFixed64Value:  req.OptionalFixed64Value,
		OptionalFloatValue:    req.OptionalFloatValue,
		OptionalDoubleValue:   req.OptionalDoubleValue,
		OptionalStatus:        req.OptionalStatus,
	}, nil
}
