package payment

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreatePayment(c *gin.Context) {
	var req CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	payment, err := h.service.CreatePayment(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrPaymentOrderIDRequired) ||
			errors.Is(err, ErrPaymentMethodRequired) ||
			errors.Is(err, ErrPaymentAmountInvalid) ||
			errors.Is(err, ErrPaymentStatusInvalid) ||
			errors.Is(err, ErrOverpaymentNotAllowed) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, payment)
}

func (h *Handler) GetPaymentsByOrder(c *gin.Context) {
	orderID := c.Param("orderId")
	if orderID == "" {
		orderID = c.Query("order_id")
	}
	if orderID == "" {
		orderID = c.Query("orderId")
	}
	payments, err := h.service.GetPaymentsByOrder(c.Request.Context(), orderID)
	if err != nil {
		if errors.Is(err, ErrPaymentOrderIDRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, payments)
}

func (h *Handler) GetDailyReport(c *gin.Context) {
	reportDate := time.Now()
	if dateQuery := c.Query("date"); dateQuery != "" {
		parsed, err := time.Parse("2006-01-02", dateQuery)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format, use YYYY-MM-DD"})
			return
		}
		reportDate = parsed
	}

	report, err := h.service.GetDailyReport(c.Request.Context(), reportDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

func (h *Handler) RefundPayment(c *gin.Context) {
	var req RefundPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.service.RefundPayment(c.Request.Context(), req.PaymentID)
	if err != nil {
		if errors.Is(err, ErrPaymentNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrRefundNotAllowed) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
