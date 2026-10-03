package wsv2

import (
	"encoding/json"
	"fmt"
)

const (
	// ProtocolVersion is the WS protocol version string.
	ProtocolVersion = "2"

	// MaxPayloadSize is the maximum allowed size for a single WS message in bytes.
	MaxPayloadSize = 8 << 20 // 8MB

	// InboundQueueCapacity is the maximum number of unprocessed inbound messages.
	InboundQueueCapacity = 128
)

// Standard JSON-RPC style error codes.
const (
	ErrNotInitialized       = -32600 // Request received before handshake completed
	ErrMethodNotFound       = -32601 // Unknown method name
	ErrInvalidParams        = -32602 // Missing or wrong-typed params
	ErrOverload             = -32001 // Inbound queue full
	ErrTurnConflict         = -32002 // Concurrent turn/start on busy thread
	ErrApprovalReleased     = -32003 // Pending approval released by abort/lock
	ErrEngineNotReady       = -32004 // Engine not connected (stub phase)
	ErrNoActiveTurn         = -32005 // Interrupt/steer on idle thread
	ErrPayloadTooLarge      = -32006 // Single message exceeds MaxPayloadSize
	ErrInternalError        = -32007 // Unexpected server error
)

// Kind identifies the four envelope types.
type Kind int

const (
	KindUnknown Kind = iota
	KindRequest
	KindNotification
	KindResponse
	KindError
)

// RequestId is a dual-type compatible id (string or int64).
type RequestId struct {
	Str string
	Int int64
	IsStr bool
}

func RequestIdFromString(s string) RequestId {
	return RequestId{Str: s, IsStr: true}
}

func RequestIdFromInt(i int64) RequestId {
	return RequestId{Int: i}
}

func (r RequestId) IsEmpty() bool {
	return !r.IsStr && r.Int == 0
}

func (r RequestId) MarshalJSON() ([]byte, error) {
	if r.IsStr {
		return json.Marshal(r.Str)
	}
	return json.Marshal(r.Int)
}

func (r *RequestId) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		r.Str = s
		r.IsStr = true
		return nil
	}
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		r.Int = i
		r.IsStr = false
		return nil
	}
	return fmt.Errorf("request id must be string or int64")
}

// ErrorPayload is the structured error in an Error message.
type ErrorPayload struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Envelope is the unified WS message envelope.
type Envelope struct {
	Id     *RequestId      `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *ErrorPayload   `json:"error,omitempty"`
}

// Kind returns the message kind based on envelope fields.
func (e *Envelope) Kind() Kind {
	if e.Error != nil {
		return KindError
	}
	if e.Id != nil {
		if e.Method != "" {
			return KindRequest
		}
		return KindResponse
	}
	return KindNotification
}

// NewResponse creates a success response envelope.
func NewResponse(id RequestId, result interface{}) (*Envelope, error) {
	var resultJSON json.RawMessage
	if result != nil {
		b, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("marshal result: %w", err)
		}
		resultJSON = b
	}
	return &Envelope{
		Id:     &id,
		Result: resultJSON,
	}, nil
}

// NewErrorResponse creates an error response envelope.
func NewErrorResponse(id RequestId, code int, message string, data interface{}) *Envelope {
	return &Envelope{
		Id: &id,
		Error: &ErrorPayload{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

// NewNotification creates a notification envelope.
func NewNotification(method string, params interface{}) (*Envelope, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	return &Envelope{
		Method: method,
		Params: b,
	}, nil
}

// DecodeEnvelope parses raw JSON bytes into an Envelope.
func DecodeEnvelope(data []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	return &env, nil
}

// EncodeEnvelope serializes an Envelope to JSON bytes.
func EncodeEnvelope(env *Envelope) ([]byte, error) {
	return json.Marshal(env)
}
