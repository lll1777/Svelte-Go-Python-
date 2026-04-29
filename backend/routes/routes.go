package routes

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"agriculture-platform/config"
	"agriculture-platform/controllers"
	"agriculture-platform/middleware"
	"agriculture-platform/models"
)

func SetupRouter(cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	userController := controllers.NewUserController(cfg)
	workOrderController := controllers.NewWorkOrderController(cfg)
	websocketController := controllers.NewWebSocketController()

	api := r.Group("/api")
	{
		auth := api.Group("/auth")
		{
			auth.POST("/register", userController.Register)
			auth.POST("/login", userController.Login)
		}

		api.GET("/health", func(c *gin.Context) {
			c.JSON(200, gin.H{"status": "ok"})
		})
	}

	protected := api.Group("")
	protected.Use(middleware.JWTAuthMiddleware(cfg))
	{
		users := protected.Group("/users")
		{
			users.GET("/profile", userController.GetProfile)
			users.PUT("/profile/farmer", middleware.RoleMiddleware(models.RoleFarmer), userController.UpdateFarmerProfile)
			users.PUT("/profile/expert", middleware.RoleMiddleware(models.RoleExpert), userController.UpdateExpertProfile)
			users.GET("/expert/:id", userController.GetExpertByID)
		}

		workOrders := protected.Group("/work-orders")
		{
			workOrders.POST("", middleware.RoleMiddleware(models.RoleFarmer, models.RoleAdmin), workOrderController.Create)
			workOrders.POST("/upload-diagnose", middleware.RoleMiddleware(models.RoleFarmer, models.RoleAdmin), workOrderController.UploadAndDiagnose)
			workOrders.GET("/my", workOrderController.GetMyWorkOrders)
			workOrders.GET("/pending", middleware.RoleMiddleware(models.RoleExpert, models.RoleAdmin), workOrderController.GetPendingWorkOrders)
			workOrders.GET("/:id", workOrderController.GetByID)
			workOrders.PATCH("/:id/status", workOrderController.UpdateStatus)
			workOrders.POST("/:id/assign", middleware.RoleMiddleware(models.RoleAdmin), workOrderController.AssignExpert)
			workOrders.POST("/:id/prescription", middleware.RoleMiddleware(models.RoleExpert, models.RoleAdmin), workOrderController.CreatePrescription)
			workOrders.POST("/:id/feedback", middleware.RoleMiddleware(models.RoleFarmer, models.RoleAdmin), workOrderController.CreateFeedback)
			workOrders.POST("/sync-offline", middleware.RoleMiddleware(models.RoleFarmer), workOrderController.SyncOfflineWorkOrders)
			workOrders.GET("/check-image-association", workOrderController.CheckImageAssociation)
		}

		ws := protected.Group("/ws")
		{
			ws.GET("", websocketController.HandleWebSocket)
		}

		messages := protected.Group("/messages")
		{
			messages.GET("/:id", websocketController.GetMessages)
		}
	}

	return r
}
