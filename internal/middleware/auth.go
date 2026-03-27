package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const authUserContextKey contextKey = "auth_user"

type AuthUser struct {
	UserID   uint
	Role     string
	BranchID string
}

type authClaims struct {
	UserID   uint   `json:"user_id"`
	Role     string `json:"role"`
	BranchID string `json:"branch_id"`
	jwt.RegisteredClaims
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			writeJSONError(w, http.StatusUnauthorized, "missing or invalid authorization header")
			return
		}

		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret()), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			writeJSONError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		authUser := AuthUser{
			UserID:   claims.UserID,
			Role:     strings.TrimSpace(claims.Role),
			BranchID: strings.TrimSpace(strings.ToLower(claims.BranchID)),
		}
		ctx := context.WithValue(r.Context(), authUserContextKey, authUser)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role string) func(http.Handler) http.Handler {
	requiredRole := strings.TrimSpace(strings.ToLower(role))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			userRole := strings.TrimSpace(strings.ToLower(authUser.Role))
			if requiredRole == "" || userRole != requiredRole {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RequireAnyRole(roles ...string) func(http.Handler) http.Handler {
	allowedRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalized := strings.TrimSpace(strings.ToLower(role))
		if normalized != "" {
			allowedRoles[normalized] = struct{}{}
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authUser, ok := UserFromContext(r.Context())
			if !ok {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			userRole := strings.TrimSpace(strings.ToLower(authUser.Role))
			if _, exists := allowedRoles[userRole]; !exists {
				writeJSONError(w, http.StatusForbidden, "forbidden")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func UserFromContext(ctx context.Context) (AuthUser, bool) {
	value := ctx.Value(authUserContextKey)
	authUser, ok := value.(AuthUser)
	return authUser, ok
}

func extractBearerToken(headerValue string) (string, bool) {
	parts := strings.SplitN(strings.TrimSpace(headerValue), " ", 2)
	if len(parts) != 2 {
		return "", false
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", false
	}
	return token, true
}

func jwtSecret() string {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if secret == "" {
		secret = "dev-secret-key"
	}
	return secret
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
