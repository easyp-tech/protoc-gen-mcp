package internal.examplemcp.kotlin

import com.google.protobuf.Empty
import internal.testproto.example.v1.Example
import internal.testproto.example.v1.ExampleAPIToolHandler
import internal.testproto.example.v1.registerExampleAPITools
import io.modelcontextprotocol.kotlin.sdk.server.ClientConnection
import io.modelcontextprotocol.kotlin.sdk.server.Server
import io.modelcontextprotocol.kotlin.sdk.server.ServerOptions
import io.modelcontextprotocol.kotlin.sdk.server.StdioServerTransport
import io.modelcontextprotocol.kotlin.sdk.types.Implementation
import io.modelcontextprotocol.kotlin.sdk.types.ServerCapabilities
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.runBlocking
import kotlinx.io.asSink
import kotlinx.io.asSource
import kotlinx.io.buffered

private const val INVALID_OUTPUT_ENV = "PROTOC_GEN_MCP_JVM_INVALID_OUTPUT"

fun main() = runBlocking {
    validateInvalidOutputMode()
    disableKotlinLoggingStartupMessage()

    val closed = CompletableDeferred<Unit>()
    val server = Server(
        Implementation(name = "protoc-gen-mcp-kotlin-example-server", version = "v0.0.1"),
        ServerOptions(ServerCapabilities(tools = ServerCapabilities.Tools(listChanged = false))),
    )
    server.onClose { closed.complete(Unit) }

    registerExampleAPITools(server, Handler(), namespace = "example")

    val transport = StdioServerTransport(
        System.`in`.asSource().buffered(),
        System.out.asSink().buffered(),
    )
    server.createSession(transport)
    closed.await()
}

private fun validateInvalidOutputMode() {
    val mode = System.getenv(INVALID_OUTPUT_ENV)
    if (mode.isNullOrEmpty()) {
        return
    }
    require(mode == "create_report") { "unsupported $INVALID_OUTPUT_ENV value: $mode" }
}

private fun invalidCreateReportOutputRequested(): Boolean =
    System.getenv(INVALID_OUTPUT_ENV) == "create_report"

private fun disableKotlinLoggingStartupMessage() {
    runCatching {
        val configClass = Class.forName("io.github.oshai.kotlinlogging.KotlinLoggingConfiguration")
        val instance = configClass.getField("INSTANCE").get(null)
        val setter = configClass.getMethod("setLogStartupMessage", Boolean::class.javaPrimitiveType)
        setter.invoke(instance, false)
    }
}

private class Handler : ExampleAPIToolHandler {
    override suspend fun createReport(
        ctx: ClientConnection,
        request: Example.CreateReportRequest,
    ): Example.CreateReportResponse {
        val response = Example.CreateReportResponse.newBuilder()
            .setReportId("report-1")
            .setTotalCount(42)
            .setStatus(Example.ReportStatus.REPORT_STATUS_OK)
        if (invalidCreateReportOutputRequested()) {
            return response.build()
        }
        return response
            .setDetails(request.details)
            .addWarnings("none")
            .build()
    }

    override suspend fun ping(
        ctx: ClientConnection,
        request: Example.PingRequest,
    ): Example.PingResponse {
        return Example.PingResponse.newBuilder()
            .setAck(Empty.getDefaultInstance())
            .build()
    }

    override suspend fun describeAdvancedShapes(
        ctx: ClientConnection,
        request: Example.DescribeAdvancedShapesRequest,
    ): Example.DescribeAdvancedShapesResponse {
        val response = Example.DescribeAdvancedShapesResponse.newBuilder()
            .putAllLabels(request.labelsMap)
            .putAllQuantities(request.quantitiesMap)
            .putAllToggles(request.togglesMap)
            .putAllLimits(request.limitsMap)

        if (request.hasObservedAt()) {
            response.setObservedAt(request.observedAt)
        }
        if (request.hasTtl()) {
            response.setTtl(request.ttl)
        }
        if (request.hasPayload()) {
            response.setPayload(request.payload)
        }
        if (request.hasItems()) {
            response.setItems(request.items)
        }
        if (request.hasDynamic()) {
            response.setDynamic(request.dynamic)
        }
        if (request.hasNote()) {
            response.setNote(request.note)
        }
        if (request.hasTotal()) {
            response.setTotal(request.total)
        }
        if (request.hasEnabled()) {
            response.setEnabled(request.enabled)
        }
        if (request.hasRatio()) {
            response.setRatio(request.ratio)
        }
        if (request.hasMask()) {
            response.setMask(request.mask)
        }
        if (request.hasBlob()) {
            response.setBlob(request.blob)
        }
        if (request.hasSmallTotal()) {
            response.setSmallTotal(request.smallTotal)
        }
        if (request.hasUintTotal()) {
            response.setUintTotal(request.uintTotal)
        }
        if (request.hasHugeTotal()) {
            response.setHugeTotal(request.hugeTotal)
        }
        if (request.hasWeight()) {
            response.setWeight(request.weight)
        }
        if (request.hasRawRatio()) {
            response.setRawRatio(request.rawRatio)
        }
        if (request.hasTree()) {
            response.setTree(request.tree)
        }
        if (request.hasDetailAny()) {
            response.setDetailAny(request.detailAny)
        }
        if (request.hasDurationAny()) {
            response.setDurationAny(request.durationAny)
        }

        when (request.selectorCase) {
            Example.DescribeAdvancedShapesRequest.SelectorCase.CITY_ALIAS -> response.setCityAlias(request.cityAlias)
            Example.DescribeAdvancedShapesRequest.SelectorCase.CITY_ID -> response.setCityId(request.cityId)
            Example.DescribeAdvancedShapesRequest.SelectorCase.CITY_DETAILS -> response.setCityDetails(request.cityDetails)
            Example.DescribeAdvancedShapesRequest.SelectorCase.SELECTOR_NOT_SET -> Unit
            null -> Unit
        }

        return response.build()
    }

    override suspend fun describeScalarShapes(
        ctx: ClientConnection,
        request: Example.DescribeScalarShapesRequest,
    ): Example.DescribeScalarShapesResponse {
        val response = Example.DescribeScalarShapesResponse.newBuilder()
            .setBoolFlag(request.boolFlag)
            .setTextValue(request.textValue)
            .setBytesValue(request.bytesValue)
            .setInt32Value(request.int32Value)
            .setSint32Value(request.sint32Value)
            .setSfixed32Value(request.sfixed32Value)
            .setUint32Value(request.uint32Value)
            .setFixed32Value(request.fixed32Value)
            .setInt64Value(request.int64Value)
            .setSint64Value(request.sint64Value)
            .setSfixed64Value(request.sfixed64Value)
            .setUint64Value(request.uint64Value)
            .setFixed64Value(request.fixed64Value)
            .setFloatValue(request.floatValue)
            .setDoubleValue(request.doubleValue)
            .setStatus(request.status)
            .setDetails(request.details)
            .addAllSamples(request.samplesList)

        if (request.hasOptionalBoolFlag()) {
            response.setOptionalBoolFlag(request.optionalBoolFlag)
        }
        if (request.hasOptionalTextValue()) {
            response.setOptionalTextValue(request.optionalTextValue)
        }
        if (request.hasOptionalBytesValue()) {
            response.setOptionalBytesValue(request.optionalBytesValue)
        }
        if (request.hasOptionalInt32Value()) {
            response.setOptionalInt32Value(request.optionalInt32Value)
        }
        if (request.hasOptionalSint32Value()) {
            response.setOptionalSint32Value(request.optionalSint32Value)
        }
        if (request.hasOptionalSfixed32Value()) {
            response.setOptionalSfixed32Value(request.optionalSfixed32Value)
        }
        if (request.hasOptionalUint32Value()) {
            response.setOptionalUint32Value(request.optionalUint32Value)
        }
        if (request.hasOptionalFixed32Value()) {
            response.setOptionalFixed32Value(request.optionalFixed32Value)
        }
        if (request.hasOptionalInt64Value()) {
            response.setOptionalInt64Value(request.optionalInt64Value)
        }
        if (request.hasOptionalSint64Value()) {
            response.setOptionalSint64Value(request.optionalSint64Value)
        }
        if (request.hasOptionalSfixed64Value()) {
            response.setOptionalSfixed64Value(request.optionalSfixed64Value)
        }
        if (request.hasOptionalUint64Value()) {
            response.setOptionalUint64Value(request.optionalUint64Value)
        }
        if (request.hasOptionalFixed64Value()) {
            response.setOptionalFixed64Value(request.optionalFixed64Value)
        }
        if (request.hasOptionalFloatValue()) {
            response.setOptionalFloatValue(request.optionalFloatValue)
        }
        if (request.hasOptionalDoubleValue()) {
            response.setOptionalDoubleValue(request.optionalDoubleValue)
        }
        if (request.hasOptionalStatus()) {
            response.setOptionalStatus(request.optionalStatus)
        }

        return response.build()
    }

    override suspend fun hiddenThing(
        ctx: ClientConnection,
        request: Example.HiddenThingRequest,
    ): Example.HiddenThingResponse {
        return Example.HiddenThingResponse.newBuilder().build()
    }
}
