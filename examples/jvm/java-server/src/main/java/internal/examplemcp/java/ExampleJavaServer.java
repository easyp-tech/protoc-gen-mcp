package internal.examplemcp.java;

import com.google.protobuf.Empty;
import io.modelcontextprotocol.json.McpJsonDefaults;
import io.modelcontextprotocol.server.McpAsyncServerExchange;
import io.modelcontextprotocol.server.transport.StdioServerTransportProvider;
import internal.testproto.example.v1.Example;
import internal.testproto.example.v1.ExampleMcp;

public final class ExampleJavaServer {
  private static final String INVALID_OUTPUT_ENV = "PROTOC_GEN_MCP_JVM_INVALID_OUTPUT";

  private ExampleJavaServer() {
  }

  public static void main(String[] args) {
    validateInvalidOutputMode();

    StdioServerTransportProvider transportProvider =
        new StdioServerTransportProvider(McpJsonDefaults.getMapper());
    ExampleMcp.registerExampleAPITools(transportProvider, new Handler(), "example");
  }

  private static void validateInvalidOutputMode() {
    String mode = System.getenv(INVALID_OUTPUT_ENV);
    if (mode == null || mode.isEmpty()) {
      return;
    }
    if (!"create_report".equals(mode)) {
      throw new IllegalArgumentException("unsupported " + INVALID_OUTPUT_ENV + " value: " + mode);
    }
  }

  private static boolean invalidCreateReportOutputRequested() {
    return "create_report".equals(System.getenv(INVALID_OUTPUT_ENV));
  }

  private static final class Handler implements ExampleMcp.ExampleAPIToolHandler {
    @Override
    public Example.CreateReportResponse createReport(
        McpAsyncServerExchange ctx,
        Example.CreateReportRequest request
    ) {
      Example.CreateReportResponse.Builder response = Example.CreateReportResponse.newBuilder()
          .setReportId("report-1")
          .setTotalCount(42)
          .setStatus(Example.ReportStatus.REPORT_STATUS_OK);
      if (invalidCreateReportOutputRequested()) {
        return response.build();
      }
      return response
          .setDetails(request.getDetails())
          .addWarnings("none")
          .build();
    }

    @Override
    public Example.PingResponse ping(
        McpAsyncServerExchange ctx,
        Example.PingRequest request
    ) {
      return Example.PingResponse.newBuilder()
          .setAck(Empty.getDefaultInstance())
          .build();
    }

    @Override
    public Example.DescribeAdvancedShapesResponse describeAdvancedShapes(
        McpAsyncServerExchange ctx,
        Example.DescribeAdvancedShapesRequest request
    ) {
      Example.DescribeAdvancedShapesResponse.Builder response =
          Example.DescribeAdvancedShapesResponse.newBuilder()
              .putAllLabels(request.getLabelsMap())
              .putAllQuantities(request.getQuantitiesMap())
              .putAllToggles(request.getTogglesMap())
              .putAllLimits(request.getLimitsMap());

      if (request.hasObservedAt()) {
        response.setObservedAt(request.getObservedAt());
      }
      if (request.hasTtl()) {
        response.setTtl(request.getTtl());
      }
      if (request.hasPayload()) {
        response.setPayload(request.getPayload());
      }
      if (request.hasItems()) {
        response.setItems(request.getItems());
      }
      if (request.hasDynamic()) {
        response.setDynamic(request.getDynamic());
      }
      if (request.hasNote()) {
        response.setNote(request.getNote());
      }
      if (request.hasTotal()) {
        response.setTotal(request.getTotal());
      }
      if (request.hasEnabled()) {
        response.setEnabled(request.getEnabled());
      }
      if (request.hasRatio()) {
        response.setRatio(request.getRatio());
      }
      if (request.hasMask()) {
        response.setMask(request.getMask());
      }
      if (request.hasBlob()) {
        response.setBlob(request.getBlob());
      }
      if (request.hasSmallTotal()) {
        response.setSmallTotal(request.getSmallTotal());
      }
      if (request.hasUintTotal()) {
        response.setUintTotal(request.getUintTotal());
      }
      if (request.hasHugeTotal()) {
        response.setHugeTotal(request.getHugeTotal());
      }
      if (request.hasWeight()) {
        response.setWeight(request.getWeight());
      }
      if (request.hasRawRatio()) {
        response.setRawRatio(request.getRawRatio());
      }
      if (request.hasTree()) {
        response.setTree(request.getTree());
      }
      if (request.hasDetailAny()) {
        response.setDetailAny(request.getDetailAny());
      }
      if (request.hasDurationAny()) {
        response.setDurationAny(request.getDurationAny());
      }

      switch (request.getSelectorCase()) {
        case CITY_ALIAS -> response.setCityAlias(request.getCityAlias());
        case CITY_ID -> response.setCityId(request.getCityId());
        case CITY_DETAILS -> response.setCityDetails(request.getCityDetails());
        case SELECTOR_NOT_SET -> {
        }
      }

      return response.build();
    }

    @Override
    public Example.DescribeScalarShapesResponse describeScalarShapes(
        McpAsyncServerExchange ctx,
        Example.DescribeScalarShapesRequest request
    ) {
      Example.DescribeScalarShapesResponse.Builder response =
          Example.DescribeScalarShapesResponse.newBuilder()
              .setBoolFlag(request.getBoolFlag())
              .setTextValue(request.getTextValue())
              .setBytesValue(request.getBytesValue())
              .setInt32Value(request.getInt32Value())
              .setSint32Value(request.getSint32Value())
              .setSfixed32Value(request.getSfixed32Value())
              .setUint32Value(request.getUint32Value())
              .setFixed32Value(request.getFixed32Value())
              .setInt64Value(request.getInt64Value())
              .setSint64Value(request.getSint64Value())
              .setSfixed64Value(request.getSfixed64Value())
              .setUint64Value(request.getUint64Value())
              .setFixed64Value(request.getFixed64Value())
              .setFloatValue(request.getFloatValue())
              .setDoubleValue(request.getDoubleValue())
              .setStatus(request.getStatus())
              .setDetails(request.getDetails())
              .addAllSamples(request.getSamplesList());

      if (request.hasOptionalBoolFlag()) {
        response.setOptionalBoolFlag(request.getOptionalBoolFlag());
      }
      if (request.hasOptionalTextValue()) {
        response.setOptionalTextValue(request.getOptionalTextValue());
      }
      if (request.hasOptionalBytesValue()) {
        response.setOptionalBytesValue(request.getOptionalBytesValue());
      }
      if (request.hasOptionalInt32Value()) {
        response.setOptionalInt32Value(request.getOptionalInt32Value());
      }
      if (request.hasOptionalSint32Value()) {
        response.setOptionalSint32Value(request.getOptionalSint32Value());
      }
      if (request.hasOptionalSfixed32Value()) {
        response.setOptionalSfixed32Value(request.getOptionalSfixed32Value());
      }
      if (request.hasOptionalUint32Value()) {
        response.setOptionalUint32Value(request.getOptionalUint32Value());
      }
      if (request.hasOptionalFixed32Value()) {
        response.setOptionalFixed32Value(request.getOptionalFixed32Value());
      }
      if (request.hasOptionalInt64Value()) {
        response.setOptionalInt64Value(request.getOptionalInt64Value());
      }
      if (request.hasOptionalSint64Value()) {
        response.setOptionalSint64Value(request.getOptionalSint64Value());
      }
      if (request.hasOptionalSfixed64Value()) {
        response.setOptionalSfixed64Value(request.getOptionalSfixed64Value());
      }
      if (request.hasOptionalUint64Value()) {
        response.setOptionalUint64Value(request.getOptionalUint64Value());
      }
      if (request.hasOptionalFixed64Value()) {
        response.setOptionalFixed64Value(request.getOptionalFixed64Value());
      }
      if (request.hasOptionalFloatValue()) {
        response.setOptionalFloatValue(request.getOptionalFloatValue());
      }
      if (request.hasOptionalDoubleValue()) {
        response.setOptionalDoubleValue(request.getOptionalDoubleValue());
      }
      if (request.hasOptionalStatus()) {
        response.setOptionalStatus(request.getOptionalStatus());
      }

      return response.build();
    }

    @Override
    public Example.HiddenThingResponse hiddenThing(
        McpAsyncServerExchange ctx,
        Example.HiddenThingRequest request
    ) {
      return Example.HiddenThingResponse.newBuilder().build();
    }
  }
}
