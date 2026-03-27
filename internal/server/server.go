package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/category"
	"backend/internal/database"
	"backend/internal/inventory"
	"backend/internal/menu"
	"backend/internal/middleware"
	"backend/internal/orders"
	"backend/internal/payment"
	"backend/internal/price"
	"backend/internal/user"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Run(addr string, db *gorm.DB) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)

	repo := auth.NewRepository(db)
	service := auth.NewService(repo, resolveJWTSecret(), resolveTokenTTL())
	handler := auth.NewHandler(service)
	auth.RegisterRoutes(mux, handler)

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)
	user.RegisterRoutes(mux, userHandler)

	pgxPool, err := database.ConnectPGXPool()
	if err != nil {
		return err
	}
	defer pgxPool.Close()

	if err := orders.EnsureSchema(context.Background(), pgxPool); err != nil {
		return err
	}
	if err := payment.EnsureSchema(context.Background(), pgxPool); err != nil {
		return err
	}
	if err := inventory.EnsureSchema(context.Background(), pgxPool); err != nil {
		return err
	}

	categoryRepo := category.NewRepository(pgxPool)
	categoryService := category.NewService(categoryRepo)
	categoryHandler := category.NewHandler(categoryService)

	menuRepo := menu.NewRepository(pgxPool)
	menuService := menu.NewService(menuRepo, categoryRepo)
	menuHandler := menu.NewHandler(menuService)

	priceRepo := price.NewRepository(pgxPool)
	priceService := price.NewService(priceRepo)
	priceHandler := price.NewHandler(priceService)

	paymentRepo := payment.NewRepository(pgxPool)
	paymentService := payment.NewService(paymentRepo)
	paymentHandler := payment.NewHandler(paymentService)

	inventoryRepo := inventory.NewRepository(pgxPool)
	inventoryService := inventory.NewService(inventoryRepo)
	inventoryHandler := inventory.NewHandler(inventoryService)

	ordersRepo := orders.NewRepository(pgxPool)
	ordersService := orders.NewServiceWithInventory(ordersRepo, inventoryService)
	ordersHandler := orders.NewHandler(ordersService)

	gin.SetMode(gin.ReleaseMode)
	categoryEngine := gin.New()
	categoryEngine.Use(gin.Recovery())
	api := categoryEngine.Group("/api")
	compat := categoryEngine.Group("")
	category.RegisterRoutes(api, categoryHandler)
	menu.RegisterRoutes(api, menuHandler)
	price.RegisterRoutes(api, priceHandler)
	payment.RegisterRoutes(api, paymentHandler)
	inventory.RegisterRoutes(api, inventoryHandler)
	orders.RegisterRoutes(api, ordersHandler)
	category.RegisterRoutes(compat, categoryHandler)
	menu.RegisterRoutes(compat, menuHandler)
	price.RegisterRoutes(compat, priceHandler)
	payment.RegisterRoutes(compat, paymentHandler)
	inventory.RegisterRoutes(compat, inventoryHandler)
	orders.RegisterRoutes(compat, ordersHandler)
	mux.Handle("/api/", categoryEngine)
	if err := os.MkdirAll(menu.UploadRootDir(), 0o755); err != nil {
		return err
	}
	uploadsRootDir := filepath.Dir(menu.UploadRootDir())
	mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(uploadsRootDir))))
	mux.Handle("/categories", categoryEngine)
	mux.Handle("/categories/", categoryEngine)
	mux.Handle("/items", categoryEngine)
	mux.Handle("/items/", categoryEngine)
	mux.Handle("/payment", categoryEngine)
	mux.Handle("/payment/", categoryEngine)
	mux.Handle("/payments", categoryEngine)
	mux.Handle("/payments/", categoryEngine)
	mux.Handle("/inventory", categoryEngine)
	mux.Handle("/inventory/", categoryEngine)
	mux.Handle("/refund", categoryEngine)
	mux.Handle("/refund/", categoryEngine)
	mux.Handle("/refunds", categoryEngine)
	mux.Handle("/refunds/", categoryEngine)

	mux.Handle("/auth/me", middleware.AuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authUser, ok := middleware.UserFromContext(r.Context())
		if !ok {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"user_id":   authUser.UserID,
			"role":      authUser.Role,
			"branch_id": authUser.BranchID,
		})
	})))

	adminHandler := middleware.AuthMiddleware(
		middleware.RequireRole("admin")(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, http.StatusOK, map[string]string{"message": "admin access granted"})
			}),
		),
	)
	mux.Handle("/admin/ping", adminHandler)

	srv := &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	return srv.ListenAndServe()
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveJWTSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = "dev-secret-key"
	}
	return secret
}

func resolveTokenTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("JWT_TTL_HOURS"))
	if raw == "" {
		return 24 * time.Hour
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(hours) * time.Hour
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
