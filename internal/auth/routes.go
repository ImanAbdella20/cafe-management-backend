package auth

import "net/http"

func RegisterRoutes(mux *http.ServeMux, handler *Handler) {
	mux.HandleFunc("/auth/register", handler.Register)
	mux.HandleFunc("/auth/login", handler.Login)
	mux.HandleFunc("/admin/login", handler.Login)
}
