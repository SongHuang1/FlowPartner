package thread

import (
	"testing"

	"github.com/songhuang/flowpartner/backend/proto"
)

func TestEventConverter_TurnStarted(t *testing.T) {
	m := NewManager()
	defer m.Close()
	conv := NewEventConverter(m)

	th, _ := m.CreateThread("t1", "main", "")
	conn := &mockConn{}
	th.AttachConn("c1", conn)

	event := &proto.AgentEvent{
		ThreadId: "t1",
		TurnId:   "u1",
		Payload: &proto.AgentEvent_TurnStarted{
			TurnStarted: &proto.TurnStarted{
				ThreadId: "t1",
				TurnId:   "u1",
			},
		},
	}

	conv.Convert(event)

	if conn.count() != 1 {
		t.Errorf("expected 1 notification, got %d", conn.count())
	}

	n := conn.notifications[0]
	if n.Method != "turn/started" {
		t.Errorf("method = %s, want turn/started", n.Method)
	}
}

func TestEventConverter_ItemDelta(t *testing.T) {
	m := NewManager()
	defer m.Close()
	conv := NewEventConverter(m)

	th, _ := m.CreateThread("t1", "main", "")
	conn := &mockConn{}
	th.AttachConn("c1", conn)

	event := &proto.AgentEvent{
		ThreadId: "t1",
		TurnId:   "u1",
		Payload: &proto.AgentEvent_ItemDelta{
			ItemDelta: &proto.ItemDelta{
				ItemId:  "i1",
				ItemType: "agentMessage",
				Delta:   "Hello",
				Seq:     1,
			},
		},
	}

	conv.Convert(event)

	if conn.count() != 1 {
		t.Fatalf("expected 1 notification, got %d", conn.count())
	}

	n := conn.notifications[0]
	if n.Method != "item/agentMessage/delta" {
		t.Errorf("method = %s, want item/agentMessage/delta", n.Method)
	}

	params, ok := n.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("params type = %T", n.Params)
	}
	if params["delta"] != "Hello" {
		t.Errorf("delta = %v, want Hello", params["delta"])
	}
	if params["seq"] != int32(1) {
		t.Errorf("seq = %v, want 1", params["seq"])
	}
}

func TestEventConverter_PermissionRequest(t *testing.T) {
	m := NewManager()
	defer m.Close()
	conv := NewEventConverter(m)

	th, _ := m.CreateThread("t1", "main", "")
	conn := &mockConn{}
	th.AttachConn("c1", conn)

	event := &proto.AgentEvent{
		ThreadId: "t1",
		TurnId:   "u1",
		Payload: &proto.AgentEvent_PermissionRequest{
			PermissionRequest: &proto.PermissionRequest{
				Tool:         "bash",
				Path:         "/usr/bin",
				Operation:    "execute",
				Detail:       "rm -rf /",
				ScopeOptions: []string{"approved", "approvedForSession", "denied"},
			},
		},
	}

	conv.Convert(event)

	if conn.count() != 1 {
		t.Fatalf("expected 1 notification, got %d", conn.count())
	}

	n := conn.notifications[0]
	if n.Method != "item/commandExecution/requestApproval" {
		t.Errorf("method = %s, want item/commandExecution/requestApproval", n.Method)
	}

	params, ok := n.Params.(map[string]interface{})
	if !ok {
		t.Fatalf("params type = %T", n.Params)
	}
	if params["tool"] != "bash" {
		t.Errorf("tool = %v, want bash", params["tool"])
	}
	if params["operation"] != "execute" {
		t.Errorf("operation = %v, want execute", params["operation"])
	}
}

func TestEventConverter_UnknownEvent(t *testing.T) {
	m := NewManager()
	defer m.Close()
	conv := NewEventConverter(m)

	th, _ := m.CreateThread("t1", "main", "")
	conn := &mockConn{}
	th.AttachConn("c1", conn)

	event := &proto.AgentEvent{
		ThreadId: "t1",
		TurnId:   "u1",
		Payload:  &proto.AgentEvent_Error{Error: &proto.Error{Message: "test error"}},
	}

	conv.Convert(event)

	if conn.count() != 1 {
		t.Errorf("expected 1 notification, got %d", conn.count())
	}

	n := conn.notifications[0]
	if n.Method != "error" {
		t.Errorf("method = %s, want error", n.Method)
	}
}

func TestEventConverter_ThreadNotFound(t *testing.T) {
	m := NewManager()
	defer m.Close()
	conv := NewEventConverter(m)

	event := &proto.AgentEvent{
		ThreadId: "nonexistent",
		TurnId:   "u1",
		Payload:  &proto.AgentEvent_Error{Error: &proto.Error{Message: "test"}},
	}

	conv.Convert(event)
}
