package payment

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrPaymentNotFound = errors.New("payment not found")
var ErrPaymentOrderIDRequired = errors.New("order id required")
var ErrOrderNotFound = errors.New("order not found")
var ErrOverpaymentNotAllowed = errors.New("overpayment not allowed")
var ErrPaymentMethodRequired = errors.New("payment method required")
var ErrPaymentAmountInvalid = errors.New("payment amount must be greater than zero")
var ErrPaymentStatusInvalid = errors.New("invalid payment status")
var ErrRefundNotAllowed = errors.New("refund not allowed")

type Service interface {
	CreatePayment(ctx context.Context, req CreatePaymentRequest) (*Payment, error)
	GetPaymentsByOrder(ctx context.Context, orderID string) ([]Payment, error)
	GetDailyReport(ctx context.Context, day time.Time) (*DailyReport, error)
	RefundPayment(ctx context.Context, paymentID int) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*Payment, error) {
	orderID := strings.TrimSpace(req.OrderID)
	if orderID == "" {
		return nil, ErrPaymentOrderIDRequired
	}

	method := strings.TrimSpace(strings.ToLower(req.Method))
	if method == "" {
		return nil, ErrPaymentMethodRequired
	}

	if req.Amount <= 0 {
		return nil, ErrPaymentAmountInvalid
	}

	status := strings.TrimSpace(strings.ToLower(req.Status))
	if status == "" {
		status = PaymentStatusPaid
	}
	if !isValidPaymentStatus(status) {
		return nil, ErrPaymentStatusInvalid
	}

	payment := &Payment{
		OrderID: orderID,
		Method:  method,
		Amount:  req.Amount,
		Status:  status,
	}

	err := s.repo.WithTx(ctx, func(txRepo Repository) error {
		orderTotal, err := txRepo.GetOrderTotal(ctx, orderID)
		if err != nil {
			return err
		}
		if orderTotal <= 0 {
			return ErrOrderNotFound
		}

		paidAmount, err := txRepo.GetTotalPaid(ctx, orderID)
		if err != nil {
			return err
		}

		if paidAmount+req.Amount > orderTotal {
			return ErrOverpaymentNotAllowed
		}

		if err := txRepo.InsertPayment(ctx, payment); err != nil {
			return err
		}

		if almostEqual(paidAmount+req.Amount, orderTotal) {
			if err := txRepo.UpdateOrderStatus(ctx, orderID, "completed"); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrOrderNotFound
				}
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return payment, nil
}

func (s *service) GetPaymentsByOrder(ctx context.Context, orderID string) ([]Payment, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return s.repo.ListAll(ctx)
	}

	return s.repo.GetByOrderID(ctx, orderID)
}

func (s *service) GetDailyReport(ctx context.Context, day time.Time) (*DailyReport, error) {
	return s.repo.GetDailyReport(ctx, day)
}

func (s *service) RefundPayment(ctx context.Context, paymentID int) error {
	if paymentID <= 0 {
		return ErrPaymentNotFound
	}

	return s.repo.WithTx(ctx, func(txRepo Repository) error {
		payment, err := txRepo.GetByID(ctx, paymentID)
		if err != nil {
			return err
		}
		if payment == nil {
			return ErrPaymentNotFound
		}
		if payment.Status != PaymentStatusPaid {
			return ErrRefundNotAllowed
		}

		if err := txRepo.RefundPayment(ctx, paymentID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrRefundNotAllowed
			}
			return err
		}

		orderTotal, err := txRepo.GetOrderTotal(ctx, payment.OrderID)
		if err != nil {
			return err
		}
		paidAmount, err := txRepo.GetTotalPaid(ctx, payment.OrderID)
		if err != nil {
			return err
		}

		status := "pending"
		if almostEqual(paidAmount, orderTotal) {
			status = "completed"
		}
		if err := txRepo.UpdateOrderStatus(ctx, payment.OrderID, status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrOrderNotFound
			}
			return err
		}

		return nil
	})
}

func isValidPaymentStatus(status string) bool {
	switch status {
	case PaymentStatusPending, PaymentStatusPaid, PaymentStatusFailed, PaymentStatusRefunded:
		return true
	default:
		return false
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= 0.000001
}
