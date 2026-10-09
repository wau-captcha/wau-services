package service

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"

	"billing/internal/biz"
	"billing/internal/conf"

	"github.com/go-kratos/kratos/v3/log"
)

// SepayIPNPath is the IPN URL path to enter in SePay (Cổng thanh toán → Cấu hình → IPN).
const SepayIPNPath = "/v1/billing/sepay/ipn"

// maxNotificationBytes bounds an IPN body, as the server's body filter does; SePay's are about 2 KB.
const maxNotificationBytes = 16 << 10

// SepayIPN receives SePay's payment notifications. It is a plain HTTP handler because SePay,
// not a WAU client, decides the request shape and authenticates with X-Secret-Key.
type SepayIPN struct {
	billing   *biz.BillingUsecase
	secretKey []byte
}

func NewSepayIPN(billing *biz.BillingUsecase, sepayConf *conf.Sepay) *SepayIPN {
	return &SepayIPN{billing: billing, secretKey: []byte(sepayConf.GetSecretKey())}
}

// ServeHTTP answers 200 only once the payment is stored, or when resending the notification
// cannot change anything (not a payment, already applied, or left for a person to settle).
// Any other failure answers 5xx, and nothing was changed, so SePay can simply send it again.
func (h *SepayIPN) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		reply(w, http.StatusMethodNotAllowed, false)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Secret-Key")), h.secretKey) != 1 {
		log.WarnContext(ctx, "sepay ipn refused: X-Secret-Key does not match the secret key", "remote", r.RemoteAddr)
		reply(w, http.StatusUnauthorized, false)
		return
	}
	var notification sepayNotification
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxNotificationBytes)).Decode(&notification); err != nil {
		log.ErrorContext(ctx, "sepay ipn unreadable", "err", err)
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			reply(w, http.StatusRequestEntityTooLarge, false)
			return
		}
		reply(w, http.StatusBadRequest, false)
		return
	}
	logged := []any{
		"notification_type", notification.NotificationType,
		"payment_code", notification.Order.OrderInvoiceNumber,
		"order_status", notification.Order.OrderStatus,
		"transaction_status", notification.Transaction.TransactionStatus,
		"payment_method", notification.Transaction.PaymentMethod,
	}
	payment, err := notification.payment()
	if errors.Is(err, errNotAPayment) {
		log.InfoContext(ctx, "sepay ipn ignored: not a captured payment", logged...)
		reply(w, http.StatusOK, true)
		return
	}
	if err != nil {
		log.ErrorContext(ctx, "sepay ipn payment unreadable", append(logged, "err", err)...)
		reply(w, http.StatusBadRequest, false)
		return
	}
	logged = append(logged, "amount", payment.Amount, "sepay_transaction_id", payment.GatewayTransactionID)

	grant, err := h.billing.ConfirmPayment(ctx, payment)
	switch {
	case errors.Is(err, biz.ErrUnknownPayment), errors.Is(err, biz.ErrAmountMismatch), errors.Is(err, biz.ErrPaymentConflict):
		// Money arrived that cannot be applied: it needs a refund or a hand-made fix.
		log.ErrorContext(ctx, "sepay payment NOT applied, settle it by hand", append(logged, "err", err)...)
		reply(w, http.StatusOK, true)
	case err != nil:
		log.ErrorContext(ctx, "sepay payment failed, SePay may send it again", append(logged, "err", err)...)
		reply(w, http.StatusInternalServerError, false)
	case grant == nil:
		log.InfoContext(ctx, "sepay payment already applied", logged...)
		reply(w, http.StatusOK, true)
	default:
		log.InfoContext(ctx, "sepay payment applied",
			append(logged, "kind", grant.Kind, "tier_id", grant.TierID, "tier_expires_at", grant.ExpiresAt)...)
		reply(w, http.StatusOK, true)
	}
}

// reply is the body SePay expects: {"success": true} acknowledges the notification.
func reply(w http.ResponseWriter, status int, success bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": success})
}
