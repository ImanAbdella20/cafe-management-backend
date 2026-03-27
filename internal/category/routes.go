package category

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	group := r.Group("/categories")

	group.GET("", middleware.GinAuthMiddleware(), handler.GetCategories)
	group.POST("", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.CreateCategory)
	group.PATCH("/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager"), handler.UpdateCategory)
	group.DELETE("/:id", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin"), handler.DeleteCategory)
}
