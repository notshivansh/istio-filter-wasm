package main

import (
	"envoyfilter/internal"
	"time"

	"github.com/akto-api-security/gomiddleware"
	"github.com/segmentio/kafka-go"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm"
	"github.com/tetratelabs/proxy-wasm-go-sdk/proxywasm/types"
)

func main() {
	proxywasm.SetVMContext(&vmContext{})
}

type vmContext struct {
	// Embed the default VM context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultVMContext
}

// NewPluginContext Override types.DefaultVMContext otherwise this plugin would do nothing :)
func (v *vmContext) NewPluginContext(contextID uint32) types.PluginContext {
	proxywasm.LogInfof("NewPluginContext context:%v", contextID)
	return &filterContext{}
}

type filterContext struct {
	// Embed the default plugin context here,
	// so that we don't need to reimplement all the methods.
	types.DefaultPluginContext
	KafkaWriter  *kafka.Writer
	kafkaContext int
}

// OnPluginStart Override types.DefaultPluginContext.
func (h *filterContext) OnPluginStart(contextID int) types.OnPluginStartStatus {
	proxywasm.LogInfof("plugin starting: %v", contextID)
	kafka_url := ""
	kafka_batch_size := 500
	kafka_batch_time_secs_duration := time.Duration(10)
	h.KafkaWriter = gomiddleware.GetKafkaWriter(kafka_url, "akto.api.logs", kafka_batch_size, kafka_batch_time_secs_duration*time.Second)
	h.kafkaContext = contextID
	return types.OnPluginStartStatusOK
}

func (h *filterContext) OnPluginDone() bool {
	proxywasm.LogInfof("plugin done.")
	return true
}

// NewHttpContext Override types.DefaultPluginContext to allow us to declare a request handler for each
// intercepted request the Envoy Sidecar sends us
func (h *filterContext) NewHttpContext(cid uint32) types.HttpContext {
	proxywasm.LogInfof("context id: %v", cid)
	temp := &internal.Handler{}
	temp.Id = cid
	temp.KafkaWriter = h.KafkaWriter
	temp.KafkaContext = h.kafkaContext
	return temp
}
