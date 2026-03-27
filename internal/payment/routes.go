package payment

import (
	"backend/internal/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.RouterGroup, handler *Handler) {
	r.GET("/payments/report", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.GetDailyReport)
	r.GET("/payments", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.GetPaymentsByOrder)
	r.GET("/payments/:orderId", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.GetPaymentsByOrder)
	r.POST("/payments", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.CreatePayment)
	r.POST("/refunds", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.RefundPayment)
	r.GET("/payment", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.GetPaymentsByOrder)
	r.GET("/payment/:orderId", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.GetPaymentsByOrder)
	r.POST("/payment", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.CreatePayment)
	r.POST("/refund", middleware.GinAuthMiddleware(), middleware.GinRequireAnyRole("admin", "manager", "cashier"), handler.RefundPayment)
}
