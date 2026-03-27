package menu

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/items", middleware.GinAuthMiddleware(), handler.GetItems)
	r.POST("/items", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.CreateItem)
	r.PATCH("/items/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.UpdateItem)
	r.DELETE("/items/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin"), handler.DeleteItem)
}
