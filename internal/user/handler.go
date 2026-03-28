package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"backend/internal/middleware"
)

type Handler struct {
	service *Service
}

type createUserRequest struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	BranchID string `json:"branch_id"`
}

type updateStatusRequest struct {
	ID       uint `json:"id"`
	IsActive bool `json:"is_active"`
}

type updateRoleRequest struct {
	ID   uint   `json:"id"`
	Role string `json:"role"`
}

type assignShiftRequest struct {
	ID        uint   `json:"id"`
	ShiftDate string `json:"shift_date"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

type currentUserProfileResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	BranchID  string `json:"branch_id"`
	IsActive  bool   `json:"is_active"`
	CreatedAt any    `json:"created_at"`
	UpdatedAt any    `json:"updated_at"`
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page := parseIntOrDefault(r.URL.Query().Get("page"), 1)
	limit := parseIntOrDefault(r.URL.Query().Get("limit"), 10)
	role := strings.TrimSpace(r.URL.Query().Get("role"))

	users, total, err := h.service.ListUsers(r.Context(), page, limit, role)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": users,
		"pagination": map[string]any{
			"page":  page,
			"limit": limit,
			"total": total,
		},
	})
}

func (h *Handler) GetUserByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := parseUintParam(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	authUser, ok := middleware.UserFromContext(r.Context())
	if !ok || authUser.UserID == 0 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	u, err := h.service.GetUserByID(r.Context(), authUser.UserID)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, currentUserProfileResponse{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		Password:  u.Password,
		Role:      u.Role,
		BranchID:  u.BranchID,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	})
}

func (h *Handler) ListShiftsByUserID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id, err := parseUintParam(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	shifts, err := h.service.ListShiftsByUserID(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": shifts})
}

func (h *Handler) ListShiftsByDate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	date := strings.TrimSpace(r.URL.Query().Get("date"))
	shifts, err := h.service.ListShiftsByDate(r.Context(), date)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"data": shifts})
}

func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": Roles()})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	name := req.Name
	if name == "" {
		name = req.FullName
	}

	u := User{
		Name:     name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
		BranchID: req.BranchID,
	}

	err := h.service.CreateUser(r.Context(), &u)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrEmailAlreadyExists):
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, u)
}

func (h *Handler) UpdateUserStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req updateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.UpdateStatus(r.Context(), req.ID, req.IsActive)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":        req.ID,
		"is_active": req.IsActive,
	})
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req updateRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	err := h.service.UpdateRole(r.Context(), req.ID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, ErrNotFound):
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":   req.ID,
		"role": req.Role,
	})
}

func (h *Handler) AssignShift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req assignShiftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	shift := Shift{
		UserID:    req.ID,
		ShiftDate: req.ShiftDate,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	}

	err := h.service.AssignShift(r.Context(), &shift)
	if err != nil {
		if errors.Is(err, ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, ErrNotFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, shift)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func parseUintParam(value string) (uint, error) {
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(parsed), nil
}

func parseIntOrDefault(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
