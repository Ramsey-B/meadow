package context

import "context"

type ContextKey string

var (
	RequestIDKey = ContextKey("X-Request-Id")
	MethodKey    = ContextKey("X-Method")
	RouteKey     = ContextKey("X-Route")
	RemoteIPKey  = ContextKey("X-Remote-Ip")
	RefererKey   = ContextKey("X-Referer")
	TenantIDKey  = ContextKey("X-Tenant-Id")
	UserIDKey    = ContextKey("X-User-Id")
)

func SetRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

func GetRequestID(ctx context.Context) string {
	value, ok := ctx.Value(RequestIDKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func GetUserID(ctx context.Context) string {
	value, ok := ctx.Value(UserIDKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetMethod(ctx context.Context, method string) context.Context {
	return context.WithValue(ctx, MethodKey, method)
}

func GetMethod(ctx context.Context) string {
	value, ok := ctx.Value(MethodKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetRoute(ctx context.Context, route string) context.Context {
	return context.WithValue(ctx, RouteKey, route)
}

func GetRoute(ctx context.Context) string {
	value, ok := ctx.Value(RouteKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetRemoteIP(ctx context.Context, remoteIP string) context.Context {
	return context.WithValue(ctx, RemoteIPKey, remoteIP)
}

func GetRemoteIP(ctx context.Context) string {
	value, ok := ctx.Value(RemoteIPKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetReferer(ctx context.Context, referer string) context.Context {
	return context.WithValue(ctx, RefererKey, referer)
}

func GetReferer(ctx context.Context) string {
	value, ok := ctx.Value(RefererKey).(string)
	if !ok {
		return ""
	}
	return value
}

func SetTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, TenantIDKey, tenantID)
}

func GetTenantID(ctx context.Context) string {
	value, ok := ctx.Value(TenantIDKey).(string)
	if !ok {
		return ""
	}
	return value
}

