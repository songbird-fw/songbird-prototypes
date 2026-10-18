package api

import (
	"io"
	"net/http"

	"github.com/agostino/firewall-backend/internal/auth"
	"github.com/agostino/firewall-backend/internal/broker"
	"github.com/agostino/firewall-backend/internal/config"
	"github.com/agostino/firewall-backend/internal/model"
	"github.com/agostino/firewall-backend/internal/service"
	"github.com/gin-gonic/gin"
)

func NewRouter(cfg *config.Config, rules *service.RuleService, authSvc *service.AuthService, jwtMgr *auth.JWTManager, br *broker.Broker) *gin.Engine {
	r := gin.Default()
	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok", "iface": cfg.XDP.Interface}) })

	v1 := r.Group("/api/v1")
	{
		authg := v1.Group("/auth")
		authg.POST("/login", func(c *gin.Context) {
			var req model.LoginRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			access, refresh, err := authSvc.Login(c.Request.Context(), req.Username, req.Password)
			if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"accessToken": access, "refreshToken": refresh})
		})
		authg.POST("/refresh", func(c *gin.Context) {
			var req model.RefreshRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			access, refresh, err := authSvc.Refresh(c.Request.Context(), req.RefreshToken)
			if err != nil { c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"accessToken": access, "refreshToken": refresh})
		})
		authg.POST("/logout", func(c *gin.Context) {
			var req model.RefreshRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			if err := authSvc.Logout(c.Request.Context(), req.RefreshToken); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
		})

		protected := v1.Group("")
		protected.Use(auth.RequireJWT(jwtMgr))
		protected.GET("/rules", func(c *gin.Context) {
			ips, err := rules.ListBlockedIPs(c.Request.Context())
			if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"blocked_ips": ips})
		})
		protected.POST("/rules/block-ip", func(c *gin.Context) {
			var req model.BlockIPRequest
			if err := c.ShouldBindJSON(&req); err != nil { c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()}); return }
			actorID, _ := c.Get("userID")
			if err := rules.BlockIP(c.Request.Context(), actorID.(string), req.IP); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusCreated, gin.H{"status": "blocked", "ip": req.IP})
		})
		protected.DELETE("/rules/block-ip/:ip", func(c *gin.Context) {
			actorID, _ := c.Get("userID")
			ip := c.Param("ip")
			if err := rules.UnblockIP(c.Request.Context(), actorID.(string), ip); err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, gin.H{"status": "unblocked", "ip": ip})
		})
		protected.GET("/stats", func(c *gin.Context) {
			stats, err := rules.Stats(c.Request.Context())
			if err != nil { c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()}); return }
			c.JSON(http.StatusOK, stats)
		})
		protected.GET("/stream/events", func(c *gin.Context) {
			ch := br.Subscribe(); defer br.Unsubscribe(ch)
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			c.Writer.Header().Set("Cache-Control", "no-cache")
			c.Writer.Header().Set("Connection", "keep-alive")
			c.Writer.Header().Set("X-Accel-Buffering", "no")
			c.Stream(func(w io.Writer) bool {
				select {
				case msg, ok := <-ch:
					if !ok { return false }
					c.SSEvent("message", msg)
					return true
				case <-c.Request.Context().Done():
					return false
				}
			})
		})
	}
	return r
}
