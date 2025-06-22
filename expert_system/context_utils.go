package expert_system

import "context"

type contextKey string

const RequestIDKey contextKey = "requestID"

// GetRequestIDFromContext retrieves a request ID from the context, if present.
func GetRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// CtxWithRequestID returns a new context with the given request ID.
func CtxWithRequestID(ctx context.Context, requestID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, RequestIDKey, requestID)
}
