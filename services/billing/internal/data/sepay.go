package data

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"billing/internal/biz"
	"billing/internal/conf"
)

// sepayGateway builds SePay checkout forms: bank transfer by QR code, in VND.
type sepayGateway struct {
	merchantID  string
	secretKey   []byte
	checkoutURL string
	returnURL   *url.URL
}

func NewPaymentGateway(sepayConf *conf.Sepay, billingConf *conf.Billing) (biz.PaymentGateway, error) {
	if _, err := absoluteURL(sepayConf.GetCheckoutUrl()); err != nil {
		return nil, fmt.Errorf("sepay checkout_url: %w", err)
	}
	returnURL, err := absoluteURL(billingConf.GetReturnUrl())
	if err != nil {
		return nil, fmt.Errorf("billing return_url: %w", err)
	}
	return &sepayGateway{
		merchantID:  sepayConf.GetMerchantId(),
		secretKey:   []byte(sepayConf.GetSecretKey()),
		checkoutURL: sepayConf.GetCheckoutUrl(),
		returnURL:   returnURL,
	}, nil
}

// CheckoutForm returns the fields in the order SePay documents them. SePay checks the signature
// against the fields in the order they are posted, so the browser must keep this order.
func (g *sepayGateway) CheckoutForm(transaction *biz.Transaction) (string, []biz.FormField) {
	fields := []biz.FormField{
		{Name: "order_amount", Value: strconv.FormatInt(transaction.Amount, 10)},
		{Name: "merchant", Value: g.merchantID},
		{Name: "currency", Value: "VND"},
		{Name: "operation", Value: "PURCHASE"},
		// Plain ASCII without commas: the signed text joins fields with commas.
		{Name: "order_description", Value: "WAU Captcha " + transaction.TierName + " " + periodText[transaction.Period]},
		{Name: "order_invoice_number", Value: transaction.PaymentCode},
		{Name: "payment_method", Value: "BANK_TRANSFER"},
		{Name: "success_url", Value: g.back(transaction, "success")},
		{Name: "error_url", Value: g.back(transaction, "error")},
		{Name: "cancel_url", Value: g.back(transaction, "cancel")},
	}
	return g.checkoutURL, append(fields, biz.FormField{Name: "signature", Value: g.sign(fields)})
}

// periodText names the period in the order description, in Vietnamese without accents.
var periodText = map[biz.Period]string{biz.PeriodMonth: "1 thang", biz.PeriodYear: "1 nam"}

// sign is SePay's form signature: base64(HMAC-SHA256("name=value,name=value,...", secret key)).
func (g *sepayGateway) sign(fields []biz.FormField) string {
	signed := make([]string, len(fields))
	for i, field := range fields {
		signed[i] = field.Name + "=" + field.Value
	}
	mac := hmac.New(sha256.New, g.secretKey)
	mac.Write([]byte(strings.Join(signed, ",")))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// back is the dashboard page the payer returns to, naming the transaction and how it went.
func (g *sepayGateway) back(transaction *biz.Transaction, result string) string {
	target := *g.returnURL
	query := target.Query()
	query.Set("transaction", transaction.ID.String())
	query.Set("result", result)
	target.RawQuery = query.Encode()
	return target.String()
}

func absoluteURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, fmt.Errorf("%q is not an absolute http(s) URL", raw)
	}
	return parsed, nil
}
