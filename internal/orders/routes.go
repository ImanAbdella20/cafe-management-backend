package orders

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	group := r.Group("/orders")

	group.GET("", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier", "barista"), handler.GetOrders)
	group.GET("/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier", "barista"), handler.GetOrderByID)
	group.POST("", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("cashier"), handler.CreateOrder)
	group.POST("/:id/items", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("cashier"), handler.AddOrderItem)
	group.PATCH("/:id/status", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier", "barista"), handler.UpdateOrderStatus)
	group.DELETE("/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin"), handler.DeleteOrder)
}
