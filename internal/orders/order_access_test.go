package orders

import (
	"context"
	"errors"
	"testing"
)

type stubGateway struct {
	verifyErr error
	verified  bool
}

func (s *stubGateway) RequestCode(context.Context, string, string) error { return nil }
func (s *stubGateway) VerifyCode(context.Context, string, string, string) error {
	s.verified = true
	return s.verifyErr
}

func TestVerifyAndRelease(t *testing.T) {
	tests := []struct {
		name       string
		phone      string
		code       string
		verifyErr  error
		wantOrder  bool
		wantVerify bool
	}{
		{name: "accepted code releases complete order", phone: "+14155550123", code: "481205", wantOrder: true, wantVerify: true},
		{name: "rejected code releases no order", phone: "+14155550123", code: "481205", verifyErr: errors.New("code rejected"), wantVerify: true},
		{name: "wrong owner stops before verification", phone: "+14155550999", code: "481205"},
		{name: "malformed code stops before verification", phone: "+14155550123", code: "12AB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gateway := &stubGateway{verifyErr: tt.verifyErr}
			order, err := NewAccess(gateway).VerifyAndRelease(context.Background(), "ORDER-1042", tt.phone, tt.code)
			if (err == nil) != tt.wantOrder {
				t.Fatalf("error = %v, wantOrder = %v", err, tt.wantOrder)
			}
			if gateway.verified != tt.wantVerify {
				t.Fatalf("verified = %v, want %v", gateway.verified, tt.wantVerify)
			}
			if tt.wantOrder && (order.Checkout.Status != "paid" || order.Fulfillment.Status != "packing" || order.Receipt.Number != "RCPT-1042") {
				t.Fatalf("released incomplete order: %+v", order)
			}
		})
	}
}
