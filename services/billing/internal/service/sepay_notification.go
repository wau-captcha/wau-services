package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"billing/internal/biz"
)

// SePay IPN values this service acts on.
const (
	notificationOrderPaid = "ORDER_PAID"
	orderStatusCaptured   = "CAPTURED"
	currencyVND           = "VND"
)

// errNotAPayment marks a notification that reports no received payment, e.g. a card void.
var errNotAPayment = errors.New("notification is not a captured payment")

// sepayNotification is the part of an IPN body the payment check reads.
type sepayNotification struct {
	NotificationType text `json:"notification_type"`
	Order            struct {
		ID                 text `json:"id"`
		OrderID            text `json:"order_id"`
		OrderStatus        text `json:"order_status"`
		OrderCurrency      text `json:"order_currency"`
		OrderAmount        text `json:"order_amount"`
		OrderInvoiceNumber text `json:"order_invoice_number"`
	} `json:"order"`
	Transaction struct {
		ID                  text `json:"id"`
		TransactionID       text `json:"transaction_id"`
		TransactionStatus   text `json:"transaction_status"`
		TransactionCurrency text `json:"transaction_currency"`
		TransactionAmount   text `json:"transaction_amount"`
		PaymentMethod       text `json:"payment_method"`
	} `json:"transaction"`
}

// payment reads the received payment out of an ORDER_PAID notification of a captured order.
// The amount is what the transaction moved, the order amount when SePay leaves it out; a
// currency other than VND is refused rather than converted.
func (n *sepayNotification) payment() (biz.Payment, error) {
	if n.NotificationType != notificationOrderPaid || n.Order.OrderStatus != orderStatusCaptured {
		return biz.Payment{}, errNotAPayment
	}
	for _, currency := range []text{n.Order.OrderCurrency, n.Transaction.TransactionCurrency} {
		if currency != "" && currency != currencyVND {
			return biz.Payment{}, fmt.Errorf("currency %q is not VND", currency)
		}
	}
	received := n.Transaction.TransactionAmount
	if received == "" {
		received = n.Order.OrderAmount
	}
	amount, err := parseVND(string(received))
	if err != nil {
		return biz.Payment{}, err
	}
	payment := biz.Payment{
		PaymentCode:          string(n.Order.OrderInvoiceNumber),
		Amount:               amount,
		GatewayTransactionID: string(firstOf(n.Transaction.ID, n.Transaction.TransactionID, n.Order.ID, n.Order.OrderID)),
	}
	if payment.PaymentCode == "" || payment.GatewayTransactionID == "" {
		return biz.Payment{}, errors.New("order_invoice_number or transaction id is missing")
	}
	return payment, nil
}

// parseVND reads a whole amount of VND written as "50000" or "50000.00".
func parseVND(value string) (int64, error) {
	whole, fraction, _ := strings.Cut(strings.TrimSpace(value), ".")
	if whole == "" || strings.Trim(whole, "0123456789") != "" || strings.Trim(fraction, "0") != "" {
		return 0, fmt.Errorf("amount %q is not a whole number of VND", value)
	}
	return strconv.ParseInt(whole, 10, 64)
}

func firstOf(values ...text) text {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// text takes a JSON string or number as its text, since SePay writes amounts and ids as
// strings but a number must not make the notification unreadable.
type text string

func (t *text) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	switch {
	case bytes.Equal(data, []byte("null")):
		*t = ""
	case len(data) > 0 && data[0] == '"':
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*t = text(strings.TrimSpace(value))
	default:
		var number json.Number
		if err := json.Unmarshal(data, &number); err != nil {
			return err
		}
		*t = text(number.String())
	}
	return nil
}
