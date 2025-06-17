package api

import (
	"net/http"
	"strings"
	"time"

	"example.com/backstage/services/device/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// RequestLogger logs HTTP requests
func RequestLogger(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		if raw != "" {
			path = path + "?" + raw
		}

		logger.WithFields(logrus.Fields{
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
			"client_ip":  clientIP,
			"method":     method,
			"path":       path,
			"user_agent": c.Request.UserAgent(),
		}).Info("HTTP Request")
	}
}

// TokenAuthentication validates access tokens
func TokenAuthentication(authService *core.AuthenticationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		token, err := authService.ValidateToken(c.Request.Context(), parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("access_token", token)
		c.Next()
	}
}

// RequireScope checks if token has required scope
func RequireScope(authService *core.AuthenticationService, scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenVal, exists := c.Get("access_token")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "no access token found"})
			c.Abort()
			return
		}

		token, ok := tokenVal.(*core.AccessToken)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid token type"})
			c.Abort()
			return
		}

		if !authService.HasScope(token, scope) {
			c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// DeviceAuthentication validates device identity
func DeviceAuthentication(deviceService *core.DeviceManagementService) gin.HandlerFunc {
	return func(c *gin.Context) {
		deviceUID := c.Param("uid")
		if deviceUID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "device uid required"})
			c.Abort()
			return
		}

		device, err := deviceService.GetDeviceByUID(c.Request.Context(), deviceUID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			c.Abort()
			return
		}

		if !device.Active {
			c.JSON(http.StatusForbidden, gin.H{"error": "device is inactive"})
			c.Abort()
			return
		}

		c.Set("device", device)
		c.Next()
	}
}

// ErrorHandler handles errors consistently
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()

			// Check if it's a business error
			if businessErr, ok := err.Err.(core.BusinessError); ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": businessErr.Message,
					"code":  businessErr.Code,
				})
				return
			}

			// Generic error
			c.JSON(c.Writer.Status(), gin.H{
				"error": err.Error(),
			})
		}
	}
}

// CORS enables cross-origin requests
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		if origin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Link")
		c.Writer.Header().Set("Access-Control-Max-Age", "300")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RateLimiter implements rate limiting for API endpoints
func RateLimiter(requestsPerMinute int) gin.HandlerFunc {
	// Simple in-memory rate limiter for now
	// TODO: use Redis-based rate limiting
	clients := make(map[string]*rateLimitClient)

	return func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		client, exists := clients[clientIP]
		if !exists {
			clients[clientIP] = &rateLimitClient{
				lastReset: now,
				requests:  1,
			}
			c.Next()
			return
		}

		// Reset counter if minute has passed
		if now.Sub(client.lastReset) > time.Minute {
			client.lastReset = now
			client.requests = 1
			c.Next()
			return
		}

		// Check rate limit
		if client.requests >= requestsPerMinute {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate limit exceeded",
				"retry_after": 60 - int(now.Sub(client.lastReset).Seconds()),
			})
			c.Abort()
			return
		}

		client.requests++
		c.Next()
	}
}

type rateLimitClient struct {
	lastReset time.Time
	requests  int
}

// Recovery handles panics and prevents server crashes
func Recovery(logger *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.WithFields(logrus.Fields{
					"error":  err,
					"path":   c.Request.URL.Path,
					"method": c.Request.Method,
				}).Error("Panic recovered")

				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
