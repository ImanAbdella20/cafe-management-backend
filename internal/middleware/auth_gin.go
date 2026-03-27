package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var catalogReadRoles = map[string]struct{}{
	"admin":   {},
	"manager": {},
	"cashier": {},
}

var catalogWriteRoles = map[string]struct{}{
	"admin":   {},
	"manager": {},
}

func GinAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret()), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		authUser := AuthUser{
			UserID:   claims.UserID,
			Role:     strings.TrimSpace(claims.Role),
			BranchID: strings.TrimSpace(strings.ToLower(claims.BranchID)),
		}

		ctx := context.WithValue(c.Request.Context(), authUserContextKey, authUser)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func GinRequireAnyRole(roles ...string) gin.HandlerFunc {
	allowedRoles := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		normalized := strings.TrimSpace(strings.ToLower(role))
		if normalized != "" {
			allowedRoles[normalized] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		authUser, ok := UserFromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		userRole := strings.TrimSpace(strings.ToLower(authUser.Role))
		if _, exists := allowedRoles[userRole]; !exists {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}

func GinRequireInventoryAccess() gin.HandlerFunc {
	return GinRequireAnyRole("admin", "manager")
}

func GinCatalogAuthorizationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization header"})
			return
		}

		claims := &authClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			return []byte(jwtSecret()), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		authUser := AuthUser{
			UserID:   claims.UserID,
			Role:     strings.TrimSpace(claims.Role),
			BranchID: strings.TrimSpace(strings.ToLower(claims.BranchID)),
		}

		ctx := context.WithValue(c.Request.Context(), authUserContextKey, authUser)
		c.Request = c.Request.WithContext(ctx)

		userRole := strings.ToLower(strings.TrimSpace(authUser.Role))
		if isCatalogWriteMethod(c.Request.Method) {
			if _, exists := catalogWriteRoles[userRole]; !exists {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		} else {
			if _, exists := catalogReadRoles[userRole]; !exists {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
				return
			}
		}

		c.Next()
	}
}

func isCatalogWriteMethod(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}
