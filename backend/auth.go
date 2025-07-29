package main

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"os"
	"time"
)

type LoginRequest struct {
	Name string `json:"name" binding:"required"`
	Role string `json:"role" binding:"required"`
}

type LoginResponse struct {
	Token string `json:"token"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func jwtSecret() []byte {
	return []byte(os.Getenv("JWT_SECRET"))
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr := c.GetHeader("Authorization")
		if len(tokenStr) > 7 && tokenStr[:7] == "Bearer " {
			tokenStr = tokenStr[7:]
		}
		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return jwtSecret(), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

func RegisterAuthRoutes(r *gin.Engine) {
	r.POST("/api/login", func(c *gin.Context) {
		var req LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and role required"})
			return
		}
		claims := jwt.MapClaims{
			"name": req.Name,
			"role": req.Role,
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenStr, _ := token.SignedString(jwtSecret())
		c.JSON(http.StatusOK, LoginResponse{Token: tokenStr, Name: req.Name, Role: req.Role})
	})
} 