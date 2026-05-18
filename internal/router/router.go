package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"opscenter/internal/handler"
	"opscenter/internal/middleware"
	"opscenter/internal/service"
)

func Setup(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// Middleware
	r.Use(middleware.CORS())

	// Serve static files (frontend)
	r.StaticFS("/assets", http.Dir("web/dist/assets"))
	r.StaticFile("/favicon.ico", "web/dist/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		c.File("web/dist/index.html")
	})

	// Services
	sshManager := service.NewSSHManager()
	previewMgr := service.NewPreviewManager()
	lockManager := service.NewLockManager()

	// Handlers
	authHandler := handler.NewAuthHandler(db)
	serverHandler := handler.NewServerHandler(db)
	logHandler := handler.NewLogHandler(db)
	lvsHandler := handler.NewLVSHandler(db, sshManager, previewMgr)
	k8sHandler := handler.NewK8sHandler(db, sshManager, previewMgr)
	preprodHandler := handler.NewPreprodHandler(db, sshManager, previewMgr, lockManager)
	nginxHandler := handler.NewNginxHandler(db, sshManager, previewMgr)
	wsHandler := handler.NewWSHandler(db, sshManager, previewMgr, lockManager)

	// Initialize admin user
	authHandler.InitAdmin()

	// Public routes
	api := r.Group("/api")
	{
		api.POST("/login", authHandler.Login)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(middleware.Auth(), middleware.UserEnabledCheck(db))
	{
		// WebSocket
		protected.GET("/ws/exec", wsHandler.Handle)

		// User info
		protected.GET("/user/info", authHandler.GetUserInfo)

		// Logs
		protected.GET("/logs", logHandler.List)

		// LVS
		lvs := protected.Group("/lvs")
		{
			lvs.GET("/list", lvsHandler.List)
			lvs.GET("/status", lvsHandler.Status)
			lvs.POST("/op/preview", lvsHandler.OpPreview)
			lvs.POST("/op/execute", lvsHandler.OpExecute)
			lvs.POST("/swap/preview", lvsHandler.SwapPreview)
			lvs.POST("/swap/execute", lvsHandler.SwapExecute)
		}

		// K8s
		k8s := protected.Group("/k8s")
		{
			k8s.GET("/rollouts", k8sHandler.Rollouts)
			k8s.POST("/online/preview", k8sHandler.OnlinePreview)
			k8s.POST("/online/execute", k8sHandler.OnlineExecute)
			k8s.POST("/sync/preview", k8sHandler.SyncPreview)
			k8s.POST("/sync/execute", k8sHandler.SyncExecute)
			k8s.POST("/rollback/preview", k8sHandler.RollbackPreview)
			k8s.POST("/rollback/execute", k8sHandler.RollbackExecute)
			k8s.POST("/full_online/preview", k8sHandler.FullOnlinePreview)
			k8s.POST("/full_online/execute", k8sHandler.FullOnlineExecute)
			k8s.POST("/full_sync/preview", k8sHandler.FullSyncPreview)
			k8s.POST("/full_sync/execute", k8sHandler.FullSyncExecute)
			k8s.POST("/full_rollback/preview", k8sHandler.FullRollbackPreview)
			k8s.POST("/full_rollback/execute", k8sHandler.FullRollbackExecute)
		}

		// Preprod
		preprod := protected.Group("/preprod")
		{
			preprod.GET("/status", preprodHandler.Status)
			preprod.POST("/scaledown/preview", preprodHandler.ScaleDownPreview)
			preprod.POST("/scaledown/execute", preprodHandler.ScaleDownExecute)
			preprod.POST("/scaleup/preview", preprodHandler.ScaleUpPreview)
			preprod.POST("/scaleup/execute", preprodHandler.ScaleUpExecute)
		}

		// Nginx
		nginx := protected.Group("/nginx")
		{
			nginx.GET("/configs", nginxHandler.Configs)
			nginx.GET("/upstreams", nginxHandler.Upstreams)
			nginx.POST("/upstream/online/preview", nginxHandler.OnlinePreview)
			nginx.POST("/upstream/online/execute", nginxHandler.OnlineExecute)
			nginx.POST("/upstream/offline/preview", nginxHandler.OfflinePreview)
			nginx.POST("/upstream/offline/execute", nginxHandler.OfflineExecute)
			nginx.POST("/upstream/swap/preview", nginxHandler.SwapPreview)
			nginx.POST("/upstream/swap/execute", nginxHandler.SwapExecute)
			nginx.POST("/upstream/toggle/preview", nginxHandler.TogglePreview)
			nginx.POST("/upstream/toggle/execute", nginxHandler.ToggleExecute)
			nginx.POST("/upstream/batch/preview", nginxHandler.BatchPreview)
			nginx.POST("/upstream/batch/execute", nginxHandler.BatchExecute)
			nginx.POST("/reload", nginxHandler.Reload)
			nginx.POST("/rollback/preview", nginxHandler.RollbackPreview)
			nginx.POST("/rollback/execute", nginxHandler.RollbackExecute)
			nginx.GET("/backups", nginxHandler.Backups)
		}

		// Server list (any authenticated user)
		protected.GET("/servers", serverHandler.List)
		protected.GET("/servers/:id", serverHandler.Get)

		// Server management (admin only)
		servers := protected.Group("/servers")
		servers.Use(middleware.AdminRequired(db))
		{
			servers.GET("/:id/edit", serverHandler.GetForEdit)
			servers.POST("", serverHandler.Create)
			servers.PUT("/:id", serverHandler.Update)
			servers.DELETE("/:id", serverHandler.Delete)
			servers.POST("/:id/test", serverHandler.TestConnection)
			servers.PUT("/:id/toggle", serverHandler.ToggleEnabled)
		}

		// User management (admin only)
		users := protected.Group("/users")
		users.Use(middleware.AdminRequired(db))
		{
			users.GET("", authHandler.ListUsers)
			users.POST("", authHandler.CreateUser)
			users.PUT("/:id", authHandler.UpdateUser)
			users.DELETE("/:id", authHandler.DeleteUser)
			users.PUT("/:id/reset-password", authHandler.ResetPassword)
			users.PUT("/:id/toggle", authHandler.ToggleUserEnabled)
		}

		// Change password (any authenticated user, for self only)
		protected.PUT("/users/:id/password", authHandler.ChangePassword)
	}

	return r
}
