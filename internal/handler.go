package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"time"

	"github.com/akto-api-security/gomiddleware"
	"github.com/segmentio/kafka-go"
	"github.com/tetratelabs/proxy-wasm-go-sdk/properties"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

type Handler struct {
	// Bring in the callback functions
	types.DefaultHttpContext
	Id                        uint32
	totalRequestBodyReadSize  int
	receivedChunks            int
	reqBody                   []byte
	totalResponseBodyReadSize int
	sentChunks                int
	resBody                   []byte
	KafkaWriter               *kafka.Writer
	KafkaContext              int
}

const (
	XRequestIdHeader = "x-request-id"
)

// OnHttpRequestHeaders is called on every request we intercept with this WASM filter
// Check out the types.HttpContext interface to see what other callbacks you can override
//
// Note: Parameters are not needed here, but a brief description:
//   - numHeaders = fairly self-explanatory, the number of request headers
//   - endOfStream = only set to false when there is a request body (e.g. in a POST/PATCH/PUT request)

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func (h *Handler) OnHttpStreamDone() {

	id := h.Id
	proxywasm.LogInfof("context from http stream: %v", id)
	proxywasm.LogInfof("WASM plugin Handling request")
	requestHeaders, err := proxywasm.GetHttpRequestHeaders()
	if err != nil {
		proxywasm.LogCriticalf("failed to get request headers: %v", err)
		// Allow Envoy Sidecar to forward this request to the upstream service
	}

	reqHeaderMap := headerArrayToMap(requestHeaders)
	// proxywasm.LogInfof("Found %v request headers", len(reqHeaderMap))
	// xRequestID := reqHeaderMap[XRequestIdHeader]
	// for key, value := range reqHeaderMap {
	// 	proxywasm.LogInfof("  %s: request header --> %s: %s", xRequestID, key, value)
	// }

	responseHeaders, err := proxywasm.GetHttpResponseHeaders()
	if err != nil {
		proxywasm.LogCriticalf("failed to get response headers: %v", err)
		// Allow Envoy Sidecar to forward this request to the upstream service
	}

	resHeaderMap := headerArrayToMap(responseHeaders)
	// proxywasm.LogInfof("Found %v response headers", len(resHeaderMap))
	// for key, value := range resHeaderMap {
	// 	proxywasm.LogInfof("  %s: response header --> %s: %s", xRequestID, key, value)
	// }

	// proxywasm.LogInfof("response body on stream done: %v", string(h.resBody[:]))
	// proxywasm.LogInfof("request body on stream done: %v", string(h.reqBody[:]))

	reqHeaderString, _ := json.Marshal(reqHeaderMap)
	respHeaderString, _ := json.Marshal(resHeaderMap)

	path, _ := properties.GetRequestPath()
	method, _ := properties.GetRequestMethod()
	protocol, _ := properties.GetRequestProtocol()
	statusCode, _ := properties.GetResponseCode()
	// status, _ := properties.GetResponseCodeDetails()
	source, _ := properties.GetDownstreamRemoteAddress()
	destination, _ := properties.GetUpstreamAddress()
	localSource, _ := properties.GetDownstreamLocalAddress()
	vxlan := hash(localSource)

	value := map[string]string{
		"path":            path,
		"requestHeaders":  string(reqHeaderString),
		"responseHeaders": string(respHeaderString),
		"method":          method,
		"requestPayload":  string(h.reqBody[:]),
		"responsePayload": string(h.resBody[:]),
		"ip":              destination,
		"time":            fmt.Sprint(time.Now().Unix()),
		"statusCode":      fmt.Sprint(statusCode),
		"type":            protocol,
		// "status":          status,
		"akto_account_id": fmt.Sprint(1000000),
		"akto_vxlan_id":   fmt.Sprint(vxlan),
		"is_pending":      "false",
		"source":          source,
	}

	out, _ := json.Marshal(value)
	// proxywasm.LogInfof("data to send: %v", value)
	proxywasm.LogInfof("req-resp.String() " + string(out))
	proxywasm.LogInfof("kafka context %v", h.KafkaContext)
	ctx := context.Background()
	go gomiddleware.Produce(h.KafkaWriter, ctx, string(out))
}

// headerArrayToMap is a simple function to convert from array of headers to a Map
func headerArrayToMap(requestHeaders [][2]string) map[string]string {
	headerMap := make(map[string]string)
	for _, header := range requestHeaders {
		headerMap[header[0]] = header[1]
	}
	return headerMap
}

func (ctx *Handler) OnHttpRequestBody(bodySize int, endOfStream bool) types.Action {

	// id := ctx.Id
	// proxywasm.LogInfof("context from request body: %v", id)
	// If some data has been received, we read it.
	// Reading the body chunk by chunk, bodySize is the size of the current chunk, not the total size of the body.
	chunkSize := bodySize - ctx.totalRequestBodyReadSize
	if chunkSize > 0 {
		ctx.receivedChunks++
		chunk, err := proxywasm.GetHttpRequestBody(ctx.totalRequestBodyReadSize, chunkSize)
		if err != nil {
			proxywasm.LogCriticalf("failed to get request body: %v", err)
			return types.ActionContinue
		}
		if len(chunk) != chunkSize {
			proxywasm.LogErrorf("read data does not match the expected size: %d != %d", len(chunk), chunkSize)
		}
		ctx.totalRequestBodyReadSize += len(chunk)
		ctx.reqBody = append(ctx.reqBody, chunk...)
	}

	// if endOfStream {
	// 	proxywasm.LogInfof("request body: %v", string(ctx.reqBody))
	// }
	return types.ActionContinue
}

func (ctx *Handler) OnHttpResponseBody(bodySize int, endOfStream bool) types.Action {

	// id := ctx.Id
	// proxywasm.LogInfof("context from response body: %v", id)

	// proxywasm.LogInfof("OnHttpResponseBody called. BodySize: %d, totalRequestBodyReadSize: %d, endOfStream: %v", bodySize, ctx.totalResponseBodyReadSize, endOfStream)

	// If some data has been received, we read it.
	// Reading the body chunk by chunk, bodySize is the size of the current chunk, not the total size of the body.

	chunkSize := bodySize - ctx.totalResponseBodyReadSize
	if chunkSize > 0 {
		ctx.sentChunks++
		chunk, err := proxywasm.GetHttpResponseBody(ctx.totalResponseBodyReadSize, chunkSize)
		// proxywasm.LogInfof("res body chunk %v", string(chunk[:]))
		if err != nil {
			proxywasm.LogCriticalf("failed to get response body: %v", err)
			return types.ActionContinue
		}
		if len(chunk) != chunkSize {
			proxywasm.LogErrorf("read data does not match the expected size: %d != %d", len(chunk), chunkSize)
		}
		ctx.totalResponseBodyReadSize += len(chunk)
		ctx.resBody = append(ctx.resBody, chunk...)
	}

	// if endOfStream {
	// 	proxywasm.LogInfof("response body: %v", string(ctx.resBody[:]))
	// }
	return types.ActionContinue
}
