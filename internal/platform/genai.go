package platform

import "go.opentelemetry.io/otel/attribute"

// GenAI semantic convention attribute keys.
// See: https://opentelemetry.io/docs/specs/semconv/gen-ai/
const (
	GenAIOperationName = attribute.Key("gen_ai.operation.name")
	GenAISystem        = attribute.Key("gen_ai.system")
	GenAIRequestModel  = attribute.Key("gen_ai.request.model")
)

// GenAISpanAttrs returns the standard GenAI semantic convention attributes
// for an Anthropic Claude invocation.
func GenAISpanAttrs(model string) []attribute.KeyValue {
	return []attribute.KeyValue{
		GenAIOperationName.String("chat"),
		GenAISystem.String("anthropic"),
		GenAIRequestModel.String(model),
	}
}

// Weave thread attribute keys.
// See: https://docs.wandb.ai/guides/weave/guides/tracking/tracing/#organize-traces-into-threads
const (
	WeaveThreadID  = attribute.Key("wandb.thread_id")
	WeaveIsTurn    = attribute.Key("wandb.is_turn")
	WeaveInputVal  = attribute.Key("input.value")
	WeaveOutputVal = attribute.Key("output.value")
)

// WeaveThreadTurnAttrs returns Weave thread attributes for a turn (outermost) span.
func WeaveThreadTurnAttrs(threadID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		WeaveThreadID.String(threadID),
		WeaveIsTurn.Bool(true),
	}
}

// WeaveThreadNestedAttrs returns Weave thread attributes for a nested (non-turn) span.
func WeaveThreadNestedAttrs(threadID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		WeaveThreadID.String(threadID),
	}
}
