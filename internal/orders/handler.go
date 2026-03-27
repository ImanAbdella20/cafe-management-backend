package orders

import (
	"backend/internal/middleware"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

var fallbackBranchID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateOrder(c *gin.Context) {
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authUser, ok := middleware.UserFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	branchID := fallbackBranchID
	if parsedBranchID, err := uuid.Parse(strings.TrimSpace(authUser.BranchID)); err == nil && parsedBranchID != uuid.Nil {
		branchID = parsedBranchID
	}

	orderType := req.OrderType
	if orderType == "" {
		orderType = "dine_in"
	}

	cashierID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("cashier:%d", authUser.UserID)))

	order, err := h.service.CreateOrder(c.Request.Context(), cashierID, branchID, orderType)
	if err != nil {
		if errors.Is(err, ErrOrderTypeRequired) || errors.Is(err, ErrOrderBranchRequired) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

func (h *Handler) GetOrders(c *gin.Context) {
	orders, err := h.service.GetOrders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

func (h *Handler) GetOrderByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	order, err := h.service.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, order)
}

func (h *Handler) AddOrderItem(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req AddOrderItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.AddItem(c.Request.Context(), orderID, req.MenuItemID, req.Qty, req.Notes)
	if err != nil {
		if errors.Is(err, ErrInvalidMenuItemID) || errors.Is(err, ErrInvalidQuantity) || errors.Is(err, ErrMenuPriceNotFound) {
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

	c.Status(http.StatusCreated)
}

func (h *Handler) UpdateOrderStatus(c *gin.Context) {
	orderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	authUser, ok := middleware.UserFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("user:%d", authUser.UserID)))

	err = h.service.UpdateStatus(c.Request.Context(), orderID, req.Status, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrOrderStatusRequired) || errors.Is(err, ErrOrderStatusInvalid) || errors.Is(err, ErrUnauthorizedStatusTransition) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (h *Handler) DeleteOrder(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid order id"})
		return
	}

	err = h.service.DeleteOrder(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
