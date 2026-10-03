package wsv2

import (
	"encoding/json"
	"testing"
)

func TestEnvelope_Kind(t *testing.T) {
	tests := []struct {
		name string
		env  Envelope
		want Kind
	}{
		{
			name: "request has method and id",
			env:  Envelope{Id: ptr(RequestIdFromInt(1)), Method: "test"},
			want: KindRequest,
		},
		{
			name: "response has id no method",
			env: Envelope{Id: ptr(RequestIdFromInt(1)), Result: json.RawMessage(`{}`)},
			want: KindResponse,
		},
		{
			name: "notification no id",
			env:  Envelope{Method: "test", Params: json.RawMessage(`{}`)},
			want: KindNotification,
		},
		{
			name: "error takes precedence",
			env:  Envelope{Id: ptr(RequestIdFromInt(1)), Error: &ErrorPayload{Code: -32600}},
			want: KindError,
		},
		{
			name: "empty envelope is notification",
			env:  Envelope{},
			want: KindNotification,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.env.Kind(); got != tt.want {
				t.Errorf("Kind() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestId_MarshalJSON(t *testing.T) {
	t.Run("string id", func(t *testing.T) {
		id := RequestIdFromString("uuid-123")
		b, err := json.Marshal(id)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `"uuid-123"` {
			t.Errorf("got %s", b)
		}
	})

	t.Run("int id", func(t *testing.T) {
		id := RequestIdFromInt(42)
		b, err := json.Marshal(id)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != `42` {
			t.Errorf("got %s", b)
		}
	})
}

func TestRequestId_UnmarshalJSON(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var id RequestId
		if err := json.Unmarshal([]byte(`"abc"`), &id); err != nil {
			t.Fatal(err)
		}
		if !id.IsStr || id.Str != "abc" {
			t.Errorf("got %+v", id)
		}
	})

	t.Run("int", func(t *testing.T) {
		var id RequestId
		if err := json.Unmarshal([]byte(`99`), &id); err != nil {
			t.Fatal(err)
		}
		if id.IsStr || id.Int != 99 {
			t.Errorf("got %+v", id)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		var id RequestId
		if err := json.Unmarshal([]byte(`true`), &id); err == nil {
			t.Error("expected error for bool")
		}
	})
}

func TestDecodeEnvelope(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		data := []byte(`{"id":"x","method":"initialize","params":{}}`)
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Kind() != KindRequest {
			t.Errorf("kind = %v", env.Kind())
		}
		if env.Method != "initialize" {
			t.Errorf("method = %s", env.Method)
		}
	})

	t.Run("notification", func(t *testing.T) {
		data := []byte(`{"method":"initialized"}`)
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Kind() != KindNotification {
			t.Errorf("kind = %v", env.Kind())
		}
	})

	t.Run("error response", func(t *testing.T) {
		data := []byte(`{"id":1,"error":{"code":-32600,"message":"Not initialized"}}`)
		env, err := DecodeEnvelope(data)
		if err != nil {
			t.Fatal(err)
		}
		if env.Kind() != KindError {
			t.Errorf("kind = %v", env.Kind())
		}
		if env.Error.Code != -32600 {
			t.Errorf("code = %d", env.Error.Code)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := DecodeEnvelope([]byte(`{bad`))
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestEncodeEnvelope(t *testing.T) {
	env := &Envelope{
		Id:     ptr(RequestIdFromInt(7)),
		Method: "test",
		Params: json.RawMessage(`{"key":"val"}`),
	}
	b, err := EncodeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["id"].(float64) != 7 {
		t.Errorf("id = %v", decoded["id"])
	}
	if decoded["method"] != "test" {
		t.Errorf("method = %v", decoded["method"])
	}
}

func TestNewErrorResponse(t *testing.T) {
	env := NewErrorResponse(RequestIdFromString("a1"), -32600, "Not initialized", nil)
	b, err := EncodeEnvelope(env)
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	errObj := decoded["error"].(map[string]interface{})
	if errObj["code"].(float64) != -32600 {
		t.Errorf("code = %v", errObj["code"])
	}
	if errObj["message"] != "Not initialized" {
		t.Errorf("message = %v", errObj["message"])
	}
}

func TestNewNotification(t *testing.T) {
	env, err := NewNotification("item/started", map[string]string{"threadId": "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if env.Method != "item/started" {
		t.Errorf("method = %s", env.Method)
	}
	if env.Id != nil {
		t.Error("notification should not have id")
	}
}

func TestMaxPayloadSize(t *testing.T) {
	if MaxPayloadSize != 8<<20 {
		t.Errorf("MaxPayloadSize = %d", MaxPayloadSize)
	}
}

func ptr[T any](v T) *T {
	return &v
}
