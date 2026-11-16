package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/zwh9616/campus-claw-plus/backend/internal/identity"
)

func businessRequest(w http.ResponseWriter, r *http.Request) (*http.Request, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	deadline, _ := ctx.Deadline()
	_ = http.NewResponseController(w).SetReadDeadline(deadline)
	// Keep the deadline while net/http drains any unread body after this
	// handler returns. Clearing it here can block a keep-alive error response.
	// net/http sets a fresh header/body deadline before the next request.
	return r.WithContext(ctx), cancel
}

func requestReadError(ctx context.Context, e error) error {
	var timeout net.Error
	if ctx.Err() != nil || errors.Is(e, identity.ErrTimeout) || errors.Is(e, context.DeadlineExceeded) || (errors.As(e, &timeout) && timeout.Timeout()) {
		return identity.ErrTimeout
	}
	return identity.ErrInvalid
}
