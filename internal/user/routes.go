package user

import (
	"net/http"

	"backend/internal/middleware"
)

func RegisterRoutes(mux *http.ServeMux, handler *Handler) {
	adminOrManager := middleware.RequireAnyRole("admin", "manager")

	usersHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListUsers(w, r)
		case http.MethodPost:
			handler.CreateUser(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})))
	updateStatusHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(handler.UpdateUserStatus)))
	updateRoleHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(handler.UpdateUserRole)))
	userByIDHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(handler.GetUserByID)))
	userShiftsHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(handler.ListShiftsByUserID)))
	shiftsHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListShiftsByDate(w, r)
		case http.MethodPost:
			handler.AssignShift(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})))
	rolesHandler := middleware.AuthMiddleware(adminOrManager(http.HandlerFunc(handler.ListRoles)))
	currentUserHandler := middleware.AuthMiddleware(http.HandlerFunc(handler.GetCurrentUser))

	mux.Handle("/users", usersHandler)
	mux.Handle("/users/me", currentUserHandler)
	mux.Handle("/users/{id}", userByIDHandler)
	mux.Handle("/users/{id}/shifts", userShiftsHandler)
	mux.Handle("/users/status", updateStatusHandler)
	mux.Handle("/users/role", updateRoleHandler)
	mux.Handle("/users/shifts", shiftsHandler)
	mux.Handle("/roles", rolesHandler)
}
