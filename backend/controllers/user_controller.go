package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"agriculture-platform/config"
	"agriculture-platform/middleware"
	"agriculture-platform/models"
	"agriculture-platform/services"
)

type UserController struct {
	userService *services.UserService
	cfg         *config.Config
}

func NewUserController(cfg *config.Config) *UserController {
	return &UserController{
		userService: services.NewUserService(),
		cfg:         cfg,
	}
}

type RegisterRequest struct {
	Username string                `json:"username" binding:"required"`
	Password string                `json:"password" binding:"required"`
	FullName string                `json:"full_name" binding:"required"`
	Phone    string                `json:"phone" binding:"required"`
	Profile  *models.FarmerProfile `json:"profile,omitempty"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  *models.User `json:"user"`
}

func (c *UserController) Register(ctx *gin.Context) {
	var req RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := c.userService.RegisterFarmer(req.Username, req.Password, req.FullName, req.Phone, req.Profile)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully",
		"user":    user,
	})
}

func (c *UserController) Login(ctx *gin.Context) {
	var req LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := c.userService.Login(req.Username, req.Password)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	token, err := middleware.GenerateToken(c.cfg, user)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	ctx.JSON(http.StatusOK, LoginResponse{
		Token: token,
		User:  user,
	})
}

func (c *UserController) GetProfile(ctx *gin.Context) {
	userID := middleware.GetCurrentUserID(ctx)

	user, err := c.userService.GetUserByID(userID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	ctx.JSON(http.StatusOK, user)
}

func (c *UserController) UpdateFarmerProfile(ctx *gin.Context) {
	userID := middleware.GetCurrentUserID(ctx)

	var profile models.FarmerProfile
	if err := ctx.ShouldBindJSON(&profile); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.userService.UpdateFarmerProfile(userID, &profile); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (c *UserController) UpdateExpertProfile(ctx *gin.Context) {
	userID := middleware.GetCurrentUserID(ctx)

	var profile models.ExpertProfile
	if err := ctx.ShouldBindJSON(&profile); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.userService.UpdateExpertProfile(userID, &profile); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (c *UserController) GetExpertByID(ctx *gin.Context) {
	expertID := ctx.Param("id")

	profile, err := c.userService.GetExpertByID(expertID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Expert not found"})
		return
	}

	user, err := c.userService.GetUserByID(expertID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Expert not found"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"profile": profile,
		"user": gin.H{
			"id":        user.ID,
			"full_name": user.FullName,
			"avatar":    user.Avatar,
		},
	})
}
