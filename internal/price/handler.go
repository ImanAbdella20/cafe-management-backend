package price

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreatePrice(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil || itemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	var req CreatePriceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.CreatePrice(c.Request.Context(), itemID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusCreated)
}

func (h *Handler) GetPricesByItem(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil || itemID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	prices, err := h.service.GetByItem(c.Request.Context(), itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prices)
}
