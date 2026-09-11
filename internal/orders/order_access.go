package orders

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var (
	phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	codePattern  = regexp.MustCompile(`^[0-9]{4,8}$`)
)

type CodeGateway interface {
	RequestCode(context.Context, string, string) error
	VerifyCode(context.Context, string, string, string) error
}

type Order struct {
	OrderID        string      `json:"order_id"`
	Phone          string      `json:"-"`
	Checkout       Checkout    `json:"checkout"`
	Fulfillment    Fulfillment `json:"fulfillment"`
	Receipt        Receipt     `json:"receipt"`
	CustomerUpdate string      `json:"customer_update"`
}

type Checkout struct {
	Status   string `json:"status"`
	Total    string `json:"total"`
	Currency string `json:"currency"`
}

type Fulfillment struct {
	Status  string `json:"status"`
	Carrier string `json:"carrier"`
}

type Receipt struct {
	Number   string `json:"number"`
	IssuedAt string `json:"issued_at"`
}

type Access struct {
	sms    CodeGateway
	orders map[string]Order
}

func NewAccess(sms CodeGateway) *Access {
	return &Access{sms: sms, orders: map[string]Order{
		"ORDER-1042": {
			OrderID: "ORDER-1042", Phone: "+14155550123",
			Checkout:       Checkout{Status: "paid", Total: "86.40", Currency: "USD"},
			Fulfillment:    Fulfillment{Status: "packing", Carrier: "Parcel North"},
			Receipt:        Receipt{Number: "RCPT-1042", IssuedAt: "2026-08-12T08:30:00Z"},
			CustomerUpdate: "Payment received; fulfillment is packing the order.",
		},
	}}
}

func (a *Access) SendCode(ctx context.Context, orderID, phone string) error {
	order, err := a.ownedOrder(orderID, phone)
	if err != nil {
		return err
	}
	return a.sms.RequestCode(ctx, order.Phone, "order-login:"+order.OrderID+":send")
}

func (a *Access) VerifyAndRelease(ctx context.Context, orderID, phone, code string) (Order, error) {
	order, err := a.ownedOrder(orderID, phone)
	if err != nil {
		return Order{}, err
	}
	if !codePattern.MatchString(code) {
		return Order{}, errors.New("code must contain 4 to 8 digits")
	}
	requestKey := fmt.Sprintf("order-login:%s:verify:%s", order.OrderID, code)
	if err := a.sms.VerifyCode(ctx, order.Phone, code, requestKey); err != nil {
		return Order{}, fmt.Errorf("verify order login: %w", err)
	}
	return order, nil
}

func (a *Access) ownedOrder(orderID, phone string) (Order, error) {
	if !phonePattern.MatchString(phone) {
		return Order{}, errors.New("phone must use E.164 format")
	}
	order, found := a.orders[orderID]
	if !found || order.Phone != phone {
		return Order{}, errors.New("order and phone do not match")
	}
	return order, nil
}
