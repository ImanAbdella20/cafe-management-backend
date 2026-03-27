package price

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	r.POST("/items/:item_id/price", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.CreatePrice)
	r.GET("/items/:item_id/prices", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.GetPricesByItem)
}
