package model

import "context"

//go:generate protoc --proto_path . --go_out=. message.proto

// ContextKey is a string that can be stored in the context
type ContextKey string

// EventHandler is the abstraction the events processor
type EventHandler interface {
	Start(ctx context.Context) error
	Stats(ctx context.Context) (HandlerStats, error)
}

// HandlerStats is the model for reporting the event processing statistics
type HandlerStats struct {
	Received int64 `json:"received"`
	Success  int64 `json:"success"`
	Errors   int64 `json:"error"`
}

// EventSink is the abstraction of the event destination
type EventSink interface {
	Save(ctx context.Context, events []*Event) error
}
