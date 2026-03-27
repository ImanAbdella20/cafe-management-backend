package payment

import "time"

const (
	PaymentStatusPending  = "pending"
	PaymentStatusPaid     = "paid"
	PaymentStatusFailed   = "failed"
	PaymentStatusRefunded = "refunded"
)

type Payment struct {
	ID        int        `json:"id"`
	OrderID   string     `json:"order_id"`
	Amount    float64    `json:"amount"`
	Method    string     `json:"payment_method"`
	Status    string     `json:"status"`
	Reference string     `json:"transaction_reference"`
	PaidAt    *time.Time `json:"paid_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type CreatePaymentRequest struct {
	OrderID string  `json:"order_id" binding:"required"`
	Method  string  `json:"method" binding:"required"`
	Amount  float64 `json:"amount" binding:"required"`
	Status  string  `json:"status"`
}

type RefundPaymentRequest struct {
	PaymentID int `json:"payment_id" binding:"required"`
}

type DailyReport struct {
	Date           string               `json:"date"`
	TotalPayments  int                  `json:"total_payments"`
	TotalCollected float64              `json:"total_collected"`
	ByMethod       []PaymentMethodTotal `json:"by_method"`
}

type PaymentMethodTotal struct {
	Method string  `json:"method"`
	Count  int     `json:"count"`
	Total  float64 `json:"total"`
}
