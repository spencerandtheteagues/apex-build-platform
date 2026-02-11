package autonomous

import "context"

type userIDKey struct{}
type projectIDKey struct{}

func withUserID(ctx context.Context, userID uint) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, userIDKey{}, userID)
}

func withProjectID(ctx context.Context, projectID *uint) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, projectIDKey{}, projectID)
}

func userIDFromContext(ctx context.Context) uint {
	if ctx == nil {
		return 0
	}
	if v := ctx.Value(userIDKey{}); v != nil {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
}

func projectIDFromContext(ctx context.Context) *uint {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value(projectIDKey{}); v != nil {
		if id, ok := v.(*uint); ok {
			return id
		}
	}
	return nil
}
