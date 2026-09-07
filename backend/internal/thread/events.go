package thread

import (
	"encoding/json"
	"log"

	"github.com/songhuang/flowpartner/backend/proto"
)

// EventConverter converts gRPC AgentEvent to WS v2 notifications.
type EventConverter struct {
	manager *Manager
}

// NewEventConverter creates a new EventConverter.
func NewEventConverter(m *Manager) *EventConverter {
	return &EventConverter{manager: m}
}

// Convert converts a gRPC AgentEvent to a WS notification and fans it out.
func (c *EventConverter) Convert(event *proto.AgentEvent) {
	threadID := event.ThreadId
	turnID := event.TurnId

	thread, ok := c.manager.GetThread(threadID)
	if !ok {
		log.Printf("[EventConverter] thread not found: %s", threadID)
		return
	}

	switch p := event.Payload.(type) {
	case *proto.AgentEvent_TurnStarted:
		c.emit(thread, "turn/started", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
		})

	case *proto.AgentEvent_TurnCompleted:
		c.emit(thread, "turn/completed", map[string]interface{}{
			"threadId":         threadID,
			"turnId":           turnID,
			"lastAgentMessage": p.TurnCompleted.LastAgentMessage,
			"durationMs":       p.TurnCompleted.DurationMs,
		})

	case *proto.AgentEvent_TurnAborted:
		c.emit(thread, "turn/interrupted", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"reason":   p.TurnAborted.Reason,
		})

	case *proto.AgentEvent_ItemStarted:
		c.emit(thread, "item/started", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"item": map[string]interface{}{
				"itemId": p.ItemStarted.ItemId,
				"type":   p.ItemStarted.ItemType,
			},
		})

	case *proto.AgentEvent_ItemDelta:
		c.emit(thread, "item/"+p.ItemDelta.ItemType+"/delta", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"itemId":   p.ItemDelta.ItemId,
			"seq":      p.ItemDelta.Seq,
			"delta":    p.ItemDelta.Delta,
		})

	case *proto.AgentEvent_ItemCompleted:
		c.emit(thread, "item/completed", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"item": map[string]interface{}{
				"itemId": p.ItemCompleted.ItemId,
				"type":   p.ItemCompleted.ItemType,
				"text":   p.ItemCompleted.Payload,
			},
		})

	case *proto.AgentEvent_UsageUpdate:
		c.emit(thread, "thread/tokenUsage/updated", map[string]interface{}{
			"threadId": threadID,
			"usage": map[string]interface{}{
				"inputTokens":       p.UsageUpdate.InputTokens,
				"cachedInputTokens": p.UsageUpdate.CachedInputTokens,
				"outputTokens":      p.UsageUpdate.OutputTokens,
				"totalTokens":       p.UsageUpdate.TotalTokens,
			},
			"modelContextWindow": p.UsageUpdate.ModelContextWindow,
		})

	case *proto.AgentEvent_PermissionRequest:
		c.handlePermissionRequest(thread, p.PermissionRequest)

	case *proto.AgentEvent_Subagent:
		c.emit(thread, "subagent/"+p.Subagent.EventType, map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"payload":  json.RawMessage(p.Subagent.Payload),
		})

	case *proto.AgentEvent_Error:
		c.emit(thread, "error", map[string]interface{}{
			"threadId": threadID,
			"turnId":   turnID,
			"message":  p.Error.Message,
		})

	default:
		log.Printf("[EventConverter] unknown event type: %T", event.Payload)
	}
}

func (c *EventConverter) emit(thread *Thread, method string, params interface{}) {
	thread.Fanout(method, params)
}

func (c *EventConverter) handlePermissionRequest(thread *Thread, req *proto.PermissionRequest) {
	decisions := req.ScopeOptions
	if len(decisions) == 0 {
		decisions = []string{"approved", "denied"}
	}

	sr := c.manager.CreateServerRequest(thread.ID, "item/commandExecution/requestApproval",
		map[string]interface{}{
			"threadId":           thread.ID,
			"tool":               req.Tool,
			"path":               req.Path,
			"operation":          req.Operation,
			"detail":             req.Detail,
			"availableDecisions": decisions,
		},
		decisions,
		KindPermission,
	)

	thread.Fanout("item/commandExecution/requestApproval", map[string]interface{}{
		"threadId":           thread.ID,
		"requestId":          sr.ID,
		"tool":               req.Tool,
		"path":               req.Path,
		"operation":          req.Operation,
		"availableDecisions": decisions,
	})
}
