package inventory

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	group := r.Group("/inventory", middleware.GinAuthMiddleware(), middleware.GinRequireInventoryAccess())

	group.GET("", handler.GetItems)
	group.GET("/:id", handler.GetItemByID)
	group.POST("", handler.CreateItem)
	group.PATCH("/:id", handler.UpdateItem)
	group.POST("/:id/adjust", handler.AdjustStock)
	group.DELETE("/:id", handler.DeleteItem)
	group.POST("/purchase-requests", handler.CreatePurchaseRequest)
	group.GET("/purchase-requests", handler.ListPurchaseRequests)
	group.POST("/purchase-requests/:id/approve", handler.ApprovePurchaseRequest)
	group.POST("/recipes", handler.UpsertMenuItemIngredient)
}
