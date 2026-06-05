// Package router 负责注册所有 HTTP 路由和中间件，初始化业务服务和处理器。
package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"

	_ "opscenter/docs"
	"opscenter/internal/handler"
	"opscenter/internal/middleware"
	"opscenter/internal/service"
)

// App 持有应用的所有组件，用于优雅停机
type App struct {
	Engine      *gin.Engine
	SSHManager  *service.SSHManager
	PreviewMgr  *service.PreviewManager
	LockManager *service.LockManager
}

// Setup 初始化路由引擎、注册所有中间件和路由，返回 App 实例。
// 路由分三组：公开路由（健康检查、登录）、WebSocket（query token 认证）、受保护路由（JWT 认证）。
// 管理员路由额外使用 AdminRequired 中间件。
func Setup(db *gorm.DB, rdb *redis.Client) *App {
	r := gin.Default()

	// Middleware
	r.Use(middleware.CORS())

	// Serve static files (frontend)
	r.StaticFS("/assets", http.Dir("web/dist/assets"))
	r.StaticFile("/favicon.svg", "web/dist/favicon.svg")
	r.NoRoute(func(c *gin.Context) {
		c.File("web/dist/index.html")
	})

	// Services
	sshManager := service.NewSSHManager()
	previewMgr := service.NewPreviewManager(rdb)
	lockManager := service.NewLockManager(rdb)
	middleware.InitBlacklist(rdb)

	// Handlers
	authHandler := handler.NewAuthHandler(db)
	serverHandler := handler.NewServerHandler(db)
	logHandler := handler.NewLogHandler(db)
	lvsHandler := handler.NewLVSHandler(db, sshManager, previewMgr)
	lvsTagHandler := handler.NewLVSTagHandler(db)
	lvsVSTagHandler := handler.NewLvsVSTagHandler(db)
	bindingHandler := handler.NewLvsPreprodBindingHandler(db)
	k8sHandler := handler.NewK8sHandler(db, sshManager, previewMgr)
	preprodHandler := handler.NewPreprodHandler(db, sshManager, previewMgr, lockManager)
	nginxHandler := handler.NewNginxHandler(db, sshManager, previewMgr)
	wsHandler := handler.NewWSHandler(db, sshManager, previewMgr, lockManager)
	dashboardHandler := handler.NewDashboardHandler(db, sshManager)

	// Initialize admin user
	authHandler.InitAdmin()

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Public routes
	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		api.POST("/login", authHandler.Login)
	}

	// Protected routes
	protected := api.Group("")
	protected.Use(middleware.Auth(), middleware.UserEnabledCheck(db))
	{
	// WebSocket (token from URL query param, verified by Auth middleware)
	protected.GET("/ws/exec", wsHandler.Handle)
		// User info
		protected.GET("/user/info", authHandler.GetUserInfo)
		protected.POST("/logout", authHandler.Logout)

		// Logs
		protected.GET("/logs", logHandler.List)

		// Dashboard
		protected.GET("/dashboard/stats", dashboardHandler.Stats)
		protected.GET("/dashboard/remote-stats", dashboardHandler.RemoteStats)
		protected.GET("/dashboard/activity-stats", dashboardHandler.ActivityStats)

		// LVS
		lvs := protected.Group("/lvs")
		{
			lvs.GET("/list", lvsHandler.List)
			lvs.GET("/status", lvsHandler.Status)
			lvs.GET("/tags", lvsTagHandler.List)
			lvs.GET("/vs_tags", lvsVSTagHandler.List)
			lvs.GET("/bindings", bindingHandler.List)
			lvs.POST("/op/preview", lvsHandler.OpPreview)
			lvs.POST("/op/execute", lvsHandler.OpExecute)
			lvs.POST("/swap/preview", lvsHandler.SwapPreview)
			lvs.POST("/swap/execute", lvsHandler.SwapExecute)
			lvs.POST("/check/scaledown", preprodHandler.CheckLvsForScaleDown)

			// Admin-only tag management
			lvsAdmin := lvs.Group("")
			lvsAdmin.Use(middleware.AdminRequired(db))
			{
				lvsAdmin.PUT("/tags", lvsTagHandler.CreateOrUpdate)
				lvsAdmin.DELETE("/tags/:vs_ip/:rs_ip", lvsTagHandler.Delete)
				lvsAdmin.PUT("/vs_tags", lvsVSTagHandler.CreateOrUpdate)
				lvsAdmin.DELETE("/vs_tags/:vs_ip", lvsVSTagHandler.Delete)
				lvsAdmin.PUT("/bindings", bindingHandler.CreateOrUpdate)
				lvsAdmin.DELETE("/bindings/:id", bindingHandler.Delete)
			}
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
			preprod.POST("/check/lvs_online", preprodHandler.CheckLvsOnline)
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
			servers.POST("/batch-delete", serverHandler.BatchDeleteServers)
			servers.POST("/batch-toggle", serverHandler.BatchToggleServers)
			servers.POST("/batch-test", serverHandler.BatchTestServers)
		}

		// User management (admin only)
		users := protected.Group("/users")
		users.Use(middleware.AdminRequired(db))
		{
			users.GET("", authHandler.ListUsers)
			users.POST("", authHandler.CreateUser)
			users.PUT("/:id", authHandler.UpdateUser)
			users.DELETE("/:id", authHandler.DeleteUser)
			users.POST("/batch-delete", authHandler.BatchDeleteUsers)
			users.POST("/batch-toggle", authHandler.BatchToggleUsers)
			users.PUT("/:id/reset-password", authHandler.ResetPassword)
			users.PUT("/:id/toggle", authHandler.ToggleUserEnabled)
			// LDAP user management
			users.GET("/ldap", authHandler.ListLDAPUsers)
			users.POST("/ldap/import", authHandler.ImportLDAPUsers)
		}

		// Change password (any authenticated user, for self only)
		protected.PUT("/users/:id/password", authHandler.ChangePassword)
	}

	return &App{
		Engine:      r,
		SSHManager:  sshManager,
		PreviewMgr:  previewMgr,
		LockManager: lockManager,
	}
}
