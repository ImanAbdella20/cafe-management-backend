package inventory

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateItem(c *gin.Context) {
	var req CreateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.service.CreateItem(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNameRequired), errors.Is(err, ErrInvalidLowStockThreshold):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, item)
}

func (h *Handler) GetItems(c *gin.Context) {
	items, err := h.service.GetItems(c.Request.Context())
	if err != nil {
		if errors.Is(err, ErrUnauthorizedInventoryAccess) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *Handler) GetItemByID(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inventory item id"})
		return
	}

	item, err := h.service.GetItemByID(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess), errors.Is(err, ErrForbiddenInventoryBranch):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inventory item id"})
		return
	}

	var req UpdateItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.service.UpdateItem(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrItemNameRequired), errors.Is(err, ErrInvalidLowStockThreshold):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, item)
}

func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inventory item id"})
		return
	}

	if err := h.service.DeleteItem(c.Request.Context(), id); err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) AdjustStock(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inventory item id"})
		return
	}

	var req AdjustStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.service.AdjustStock(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrBranchIDRequired),
			errors.Is(err, ErrMovementTypeRequired),
			errors.Is(err, ErrInvalidStockQuantity),
			errors.Is(err, ErrInsufficientStock):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess), errors.Is(err, ErrForbiddenInventoryBranch):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) CreatePurchaseRequest(c *gin.Context) {
	var req CreatePurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	purchase, err := h.service.CreatePurchaseRequest(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrItemNotFound), errors.Is(err, ErrBranchIDRequired), errors.Is(err, ErrInvalidStockQuantity):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess), errors.Is(err, ErrForbiddenInventoryBranch):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, purchase)
}

func (h *Handler) ListPurchaseRequests(c *gin.Context) {
	var branchID *uuid.UUID
	if rawBranch := c.Query("branch_id"); rawBranch != "" {
		parsedBranch, err := uuid.Parse(rawBranch)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid branch_id"})
			return
		}
		branchID = &parsedBranch
	}

	requests, err := h.service.ListPurchaseRequests(c.Request.Context(), branchID, c.Query("status"))
	if err != nil {
		switch {
		case errors.Is(err, ErrUnauthorizedInventoryAccess), errors.Is(err, ErrForbiddenInventoryBranch):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, requests)
}

func (h *Handler) ApprovePurchaseRequest(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid purchase request id"})
		return
	}

	var req ApprovePurchaseRequest
	_ = c.ShouldBindJSON(&req)

	result, err := h.service.ApprovePurchaseRequest(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrPurchaseRequestNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, ErrPurchaseRequestNotPending), errors.Is(err, ErrInsufficientStock):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess), errors.Is(err, ErrForbiddenInventoryBranch):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *Handler) UpsertMenuItemIngredient(c *gin.Context) {
	var req UpsertMenuItemIngredientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpsertMenuItemIngredient(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, ErrInvalidMenuItemID), errors.Is(err, ErrInvalidQuantityPerOrder), errors.Is(err, ErrItemNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUnauthorizedInventoryAccess):
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.Status(http.StatusCreated)
}
