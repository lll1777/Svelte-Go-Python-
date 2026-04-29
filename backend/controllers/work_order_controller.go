package controllers

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"agriculture-platform/config"
	"agriculture-platform/middleware"
	"agriculture-platform/models"
	"agriculture-platform/services"
)

type WorkOrderController struct {
	workOrderService *services.WorkOrderService
	imageService     *services.ImageService
	webSocketService *services.WebSocketService
	userService      *services.UserService
	cfg              *config.Config
}

func NewWorkOrderController(cfg *config.Config) *WorkOrderController {
	return &WorkOrderController{
		workOrderService: services.NewWorkOrderService(),
		imageService:     services.NewImageService(cfg),
		webSocketService: services.GetWebSocketService(),
		userService:      services.NewUserService(),
		cfg:              cfg,
	}
}

type CreateWorkOrderRequest struct {
	Title         string  `json:"title" binding:"required"`
	Description   string  `json:"description" binding:"required"`
	CropType      string  `json:"crop_type" binding:"required"`
	Location      string  `json:"location"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Priority      int     `json:"priority"`
	IsOfflineCreated bool `json:"is_offline_created"`
}

func (c *WorkOrderController) Create(ctx *gin.Context) {
	farmerID := middleware.GetCurrentUserID(ctx)

	var req CreateWorkOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workOrder := &models.WorkOrder{
		Title:         req.Title,
		Description:   req.Description,
		CropType:      req.CropType,
		Location:      req.Location,
		Latitude:      req.Latitude,
		Longitude:     req.Longitude,
		Priority:      req.Priority,
		IsOfflineCreated: req.IsOfflineCreated,
	}

	wo, err := c.workOrderService.CreateWorkOrder(workOrder, farmerID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, wo)
}

func (c *WorkOrderController) UploadAndDiagnose(ctx *gin.Context) {
	farmerID := middleware.GetCurrentUserID(ctx)

	form, err := ctx.MultipartForm()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := ctx.PostForm("title")
	description := ctx.PostForm("description")
	cropType := ctx.PostForm("crop_type")
	location := ctx.PostForm("location")
	latitude, _ := strconv.ParseFloat(ctx.PostForm("latitude"), 64)
	longitude, _ := strconv.ParseFloat(ctx.PostForm("longitude"), 64)

	workOrder := &models.WorkOrder{
		Title:       title,
		Description: description,
		CropType:    cropType,
		Location:    location,
		Latitude:    latitude,
		Longitude:   longitude,
		Status:      models.StatusDiagnosing,
	}

	wo, err := c.workOrderService.CreateWorkOrder(workOrder, farmerID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	files := form.File["images"]
	var imageHashes []string
	var primaryImageHash string

	for i, file := range files {
		fileContent, err := file.Open()
		if err != nil {
			continue
		}
		defer fileContent.Close()

		imageData, err := io.ReadAll(fileContent)
		if err != nil {
			continue
		}

		diagnosisResp, err := c.imageService.DiagnoseImage(imageData, file.Filename, cropType)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Image diagnosis failed: " + err.Error()})
			return
		}

		imageURL := "/uploads/" + uuid.New().String() + "_" + file.Filename
		imageHash := diagnosisResp.ImageHash

		image := &models.WorkOrderImage{
			WorkOrderID: wo.ID,
			ImageURL:    imageURL,
			ImageHash:   imageHash,
			IsPrimary:   i == 0,
			Location:    location,
			Latitude:    latitude,
			Longitude:   longitude,
		}

		c.workOrderService.AddWorkOrderImage(image)

		imageHashes = append(imageHashes, imageHash)
		if i == 0 {
			primaryImageHash = imageHash

			diagnosisResult := c.imageService.DiagnosisResultToModel(diagnosisResp)
			diagnosisResult.WorkOrderID = wo.ID
			c.workOrderService.SaveDiagnosisResult(wo.ID, diagnosisResult)

			wo.DiagnosisResult = diagnosisResult
			wo.AIConfidence = diagnosisResp.Confidence
		}
	}

	expert, err := c.workOrderService.FindNearestExpert(latitude, longitude, cropType)
	if err == nil {
		c.workOrderService.AssignExpert(wo.ID, expert.UserID, farmerID)
		c.webSocketService.SendNewWorkOrderNotification(expert.UserID, wo)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"work_order":      wo,
		"image_hashes":    imageHashes,
		"primary_image_hash": primaryImageHash,
	})
}

func (c *WorkOrderController) GetByID(ctx *gin.Context) {
	workOrderID := ctx.Param("id")

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Work order not found"})
		return
	}

	userID := middleware.GetCurrentUserID(ctx)
	userRole := middleware.GetCurrentUserRole(ctx)

	if userRole != string(models.RoleAdmin) {
		if wo.FarmerID != userID && (wo.ExpertID == nil || *wo.ExpertID != userID) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	images, _ := c.workOrderService.GetWorkOrderImages(workOrderID)

	ctx.JSON(http.StatusOK, gin.H{
		"work_order": wo,
		"images":     images,
	})
}

func (c *WorkOrderController) GetMyWorkOrders(ctx *gin.Context) {
	userID := middleware.GetCurrentUserID(ctx)
	userRole := middleware.GetCurrentUserRole(ctx)

	status := ctx.DefaultQuery("status", "")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	var workOrders []models.WorkOrder
	var total int64
	var err error

	if userRole == string(models.RoleFarmer) {
		workOrders, total, err = c.workOrderService.GetWorkOrdersByFarmer(userID, status, page, pageSize)
	} else if userRole == string(models.RoleExpert) {
		workOrders, total, err = c.workOrderService.GetWorkOrdersByExpert(userID, status, page, pageSize)
	} else {
		workOrders, total, err = c.workOrderService.GetPendingWorkOrders(page, pageSize)
	}

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":       workOrders,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

func (c *WorkOrderController) GetPendingWorkOrders(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	workOrders, total, err := c.workOrderService.GetPendingWorkOrders(page, pageSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"data":       workOrders,
		"total":      total,
		"page":       page,
		"page_size":  pageSize,
	})
}

type UpdateStatusRequest struct {
	NewStatus string `json:"new_status" binding:"required"`
	Reason    string `json:"reason"`
}

func (c *WorkOrderController) UpdateStatus(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	userID := middleware.GetCurrentUserID(ctx)

	var req UpdateStatusRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newStatus := models.WorkOrderStatus(req.NewStatus)

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Work order not found"})
		return
	}

	userRole := middleware.GetCurrentUserRole(ctx)
	if userRole != string(models.RoleAdmin) {
		if wo.FarmerID != userID && (wo.ExpertID == nil || *wo.ExpertID != userID) {
			ctx.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}
	}

	if err := c.workOrderService.UpdateStatus(workOrderID, newStatus, userID, req.Reason); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.webSocketService.SendStatusUpdate(workOrderID, newStatus, req.Reason)

	ctx.JSON(http.StatusOK, gin.H{"message": "Status updated successfully"})
}

type AssignExpertRequest struct {
	ExpertID string `json:"expert_id" binding:"required"`
}

func (c *WorkOrderController) AssignExpert(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	assignerID := middleware.GetCurrentUserID(ctx)

	var req AssignExpertRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.workOrderService.AssignExpert(workOrderID, req.ExpertID, assignerID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	wo, _ := c.workOrderService.GetWorkOrderByID(workOrderID)
	c.webSocketService.SendNewWorkOrderNotification(req.ExpertID, wo)

	ctx.JSON(http.StatusOK, gin.H{"message": "Expert assigned successfully"})
}

type CreatePrescriptionRequest struct {
	Diagnosis        string   `json:"diagnosis" binding:"required"`
	TreatmentPlan    string   `json:"treatment_plan"`
	Medications      string   `json:"medications"`
	Dosage           string   `json:"dosage"`
	ApplicationMethod string  `json:"application_method"`
	PreventionTips   string   `json:"prevention_tips"`
	Notes            string   `json:"notes"`
}

func (c *WorkOrderController) CreatePrescription(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	expertID := middleware.GetCurrentUserID(ctx)

	var req CreatePrescriptionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Work order not found"})
		return
	}

	if wo.ExpertID == nil || *wo.ExpertID != expertID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only assigned expert can create prescription"})
		return
	}

	if req.Medications != "" {
		var medications []string
		json.Unmarshal([]byte(req.Medications), &medications)
		if len(medications) > 0 {
			checkResult, err := c.imageService.CheckPrescriptionCompatibility(medications)
			if err == nil && !checkResult.IsSafe {
				ctx.JSON(http.StatusBadRequest, gin.H{
					"error":       "Medication compatibility issues detected",
					"warnings":    checkResult.Warnings,
					"suggestions": checkResult.Suggestions,
				})
				return
			}
		}
	}

	prescription := &models.Prescription{
		Diagnosis:         req.Diagnosis,
		TreatmentPlan:     req.TreatmentPlan,
		Medications:       req.Medications,
		Dosage:            req.Dosage,
		ApplicationMethod: req.ApplicationMethod,
		PreventionTips:    req.PreventionTips,
		Notes:             req.Notes,
	}

	if err := c.workOrderService.CreatePrescription(workOrderID, prescription, expertID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.webSocketService.SendPrescriptionNotification(workOrderID, wo.FarmerID, prescription)

	ctx.JSON(http.StatusOK, gin.H{
		"message":      "Prescription created successfully",
		"prescription": prescription,
	})
}

type CreateFeedbackRequest struct {
	Rating        int    `json:"rating" binding:"required"`
	Effectiveness string `json:"effectiveness"`
	Comments      string `json:"comments"`
	Improvements  string `json:"improvements"`
	IsSolved      bool   `json:"is_solved"`
}

func (c *WorkOrderController) CreateFeedback(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	farmerID := middleware.GetCurrentUserID(ctx)

	var req CreateFeedbackRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "Work order not found"})
		return
	}

	if wo.FarmerID != farmerID {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Only the farmer who created the work order can submit feedback"})
		return
	}

	feedback := &models.Feedback{
		Rating:        req.Rating,
		Effectiveness: req.Effectiveness,
		Comments:      req.Comments,
		Improvements:  req.Improvements,
		IsSolved:      req.IsSolved,
	}

	if err := c.workOrderService.CreateFeedback(workOrderID, feedback, farmerID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message":  "Feedback submitted successfully",
		"feedback": feedback,
	})
}

func (c *WorkOrderController) SyncOfflineWorkOrders(ctx *gin.Context) {
	farmerID := middleware.GetCurrentUserID(ctx)

	var offlineOrders []models.WorkOrder
	if err := ctx.ShouldBindJSON(&offlineOrders); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.workOrderService.SyncOfflineWorkOrders(farmerID, offlineOrders); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Offline work orders synced successfully"})
}

func (c *WorkOrderController) CheckImageAssociation(ctx *gin.Context) {
	imageHash := ctx.Query("image_hash")

	if strings.TrimSpace(imageHash) == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "image_hash is required"})
		return
	}

	db := services.NewWorkOrderService()
	var images []models.WorkOrderImage

	if err := db.Where("image_hash = ?", imageHash).Find(&images).Error; err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if len(images) == 0 {
		ctx.JSON(http.StatusOK, gin.H{
			"is_associated": false,
			"message":       "Image not associated with any work order",
		})
		return
	}

	var workOrders []map[string]interface{}
	for _, img := range images {
		wo, err := c.workOrderService.GetWorkOrderByID(img.WorkOrderID)
		if err == nil {
			workOrders = append(workOrders, map[string]interface{}{
				"work_order_id": wo.ID,
				"title":         wo.Title,
				"status":        wo.Status,
				"is_primary":    img.IsPrimary,
			})
		}
	}

	ctx.JSON(http.StatusOK, gin.H{
		"is_associated": true,
		"image_hash":    imageHash,
		"work_orders":   workOrders,
		"count":         len(images),
	})
}
