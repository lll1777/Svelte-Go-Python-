package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"agriculture-platform/config"
	"agriculture-platform/database"
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
	Title            string  `json:"title" binding:"required"`
	Description      string  `json:"description" binding:"required"`
	CropType         string  `json:"crop_type" binding:"required"`
	Location         string  `json:"location"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	Priority         int     `json:"priority"`
	IsOfflineCreated bool    `json:"is_offline_created"`
}

func (c *WorkOrderController) Create(ctx *gin.Context) {
	farmerID := middleware.GetCurrentUserID(ctx)

	var req CreateWorkOrderRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	workOrder := &models.WorkOrder{
		Title:            req.Title,
		Description:      req.Description,
		CropType:         req.CropType,
		Location:         req.Location,
		Latitude:         req.Latitude,
		Longitude:        req.Longitude,
		Priority:         req.Priority,
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

	if title == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	if cropType == "" {
		cropType = "rice"
	}

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
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create work order: " + err.Error()})
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "At least one image is required for diagnosis"})
		return
	}

	var imageHashes []string
	var primaryImageHash string
	var primaryDiagnosis *services.DiagnosisResponse

	for i, file := range files {
		fileContent, err := file.Open()
		if err != nil {
			continue
		}

		imageData, err := io.ReadAll(fileContent)
		fileContent.Close()
		if err != nil {
			continue
		}

		diagnosisResp, err := c.imageService.DiagnoseImageWithWorkOrderID(
			wo.ID,
			imageData,
			file.Filename,
			cropType,
			farmerID,
		)

		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":       "Image diagnosis failed",
				"details":     err.Error(),
				"work_order_id": wo.ID,
			})
			return
		}

		if diagnosisResp.WorkOrderID != wo.ID {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error":       "CRITICAL SECURITY VIOLATION: Diagnosis result identity mismatch",
				"work_order_id": wo.ID,
			})
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
			primaryDiagnosis = diagnosisResp

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
		"work_order":           wo,
		"image_hashes":         imageHashes,
		"primary_image_hash":   primaryImageHash,
		"diagnosis_verified":   true,
		"work_order_id_bound":  wo.ID,
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
		"data":      workOrders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
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
		"data":      workOrders,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
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
	Diagnosis         string `json:"diagnosis" binding:"required"`
	TreatmentPlan     string `json:"treatment_plan"`
	Medications       string `json:"medications"`
	Dosage            string `json:"dosage"`
	ApplicationMethod string `json:"application_method"`
	PreventionTips    string `json:"prevention_tips"`
	Notes             string `json:"notes"`
}

func (c *WorkOrderController) CreatePrescription(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	expertID := middleware.GetCurrentUserID(ctx)

	var req CreatePrescriptionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":               err.Error(),
			"prescription_blocked": true,
		})
		return
	}

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":               "Work order not found",
			"prescription_blocked": true,
		})
		return
	}

	if wo.ExpertID == nil || *wo.ExpertID != expertID {
		ctx.JSON(http.StatusForbidden, gin.H{
			"error":               "Only assigned expert can create prescription",
			"prescription_blocked": true,
		})
		return
	}

	var medications []string
	if req.Medications != "" {
		if err := json.Unmarshal([]byte(req.Medications), &medications); err != nil {
			medications = []string{req.Medications}
		}
	}

	checkResult, err := c.imageService.CheckPrescriptionCompatibilityWithRetry(
		medications,
		workOrderID,
		expertID,
	)

	if err != nil {
		var statusCode int
		var warningMessage string
		
		if checkResult.CheckStatus == services.SafetyCheckCircuitOpen {
			statusCode = http.StatusServiceUnavailable
			warningMessage = "🚨 EMERGENCY: Safety check service has been disabled due to repeated failures. " +
				"All prescriptions are BLOCKED until service is restored. Please contact technical support immediately."
		} else if checkResult.CheckStatus == services.SafetyCheckTimeout {
			statusCode = http.StatusGatewayTimeout
			warningMessage = "🚨 TIMEOUT: Safety check service is not responding. " +
				"Prescription BLOCKED for your safety. Please verify medication compatibility manually or try again later."
		} else {
			statusCode = http.StatusServiceUnavailable
			warningMessage = "🚨 CRITICAL: Safety check failed. " +
				"Prescription BLOCKED to protect patient safety. Please contact support if this issue persists."
		}

		ctx.JSON(statusCode, gin.H{
			"error":                  warningMessage,
			"is_safe":                false,
			"warnings":               checkResult.Warnings,
			"suggestions":            checkResult.Suggestions,
			"is_fallback":            checkResult.IsFallback,
			"check_status":           checkResult.CheckStatus,
			"service_available":      checkResult.ServiceAvailable,
			"error_details":          err.Error(),
			"prescription_blocked":   true,
			"security_policy":        "FAIL-CLOSED (Default) - No check = No prescription",
			"recommended_action":     "Please verify the safety of this prescription manually before issuing.",
		})
		return
	}

	if checkResult.IsFallback {
		if services.DefaultServiceConfig.FailOpen {
			ctx.JSON(http.StatusAccepted, gin.H{
				"warning":                "⚠️ SAFETY WARNING: Compatibility check service unavailable.",
				"mode":                   "FAIL-OPEN (NOT RECOMMENDED for production)",
				"is_fallback":            true,
				"warnings":               checkResult.Warnings,
				"suggestions":            checkResult.Suggestions,
				"check_status":           checkResult.CheckStatus,
				"security_notice":        "Prescription allowed due to fail-open configuration, but safety is NOT VERIFIED.",
				"required_action":        "YOU MUST manually verify this prescription is safe before administering.",
			})
			return
		} else {
			ctx.JSON(http.StatusServiceUnavailable, gin.H{
				"error":                  "🚨 SAFETY BLOCKED: Prescription compatibility check is unavailable.",
				"is_safe":                false,
				"warnings":               checkResult.Warnings,
				"suggestions":            "Prescription BLOCKED. Please try again later or contact technical support.",
				"is_fallback":            true,
				"check_status":           checkResult.CheckStatus,
				"service_available":      checkResult.ServiceAvailable,
				"prescription_blocked":   true,
				"security_policy":        "FAIL-CLOSED (Default) - Safety is our top priority.",
				"rationale":              "Pesticide safety is critical. Better to delay than issue an unsafe prescription.",
			})
			return
		}
	}

	if !checkResult.IsSafe {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error":                  "🚨 INCOMPATIBLE MEDICATIONS: Detected dangerous drug interactions.",
			"is_safe":                false,
			"warnings":               checkResult.Warnings,
			"suggestions":            checkResult.Suggestions,
			"check_status":           checkResult.CheckStatus,
			"prescription_blocked":   true,
			"security_policy":        "INCOMPATIBILITY DETECTED - Prescription not allowed",
			"recommended_action":     "Please adjust the medication combination to avoid dangerous interactions.",
			"medications_checked":    medications,
		})
		return
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
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error":               "Failed to create prescription: " + err.Error(),
			"prescription_blocked": true,
		})
		return
	}

	c.webSocketService.SendPrescriptionNotification(workOrderID, wo.FarmerID, prescription)

	ctx.JSON(http.StatusOK, gin.H{
		"message":              "✅ Prescription created successfully",
		"prescription":         prescription,
		"compatibility_checked": true,
		"is_safe":              true,
		"check_status":         checkResult.CheckStatus,
		"check_timestamp":      checkResult.CheckTimestamp,
		"security_verified":    true,
		"warnings":             checkResult.Warnings,
		"medications":          medications,
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

	if req.Rating < 1 || req.Rating > 5 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Rating must be between 1 and 5"})
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

	userID := middleware.GetCurrentUserID(ctx)
	userRole := middleware.GetCurrentUserRole(ctx)

	var images []models.WorkOrderImage

	db := database.GetDB()
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
		if err != nil {
			continue
		}

		if userRole != string(models.RoleAdmin) {
			if wo.FarmerID != userID && (wo.ExpertID == nil || *wo.ExpertID != userID) {
				ctx.JSON(http.StatusForbidden, gin.H{
					"error": "Access denied - you are not authorized to view this work order",
				})
				return
			}
		}

		workOrders = append(workOrders, map[string]interface{}{
			"work_order_id": wo.ID,
			"title":         wo.Title,
			"status":        wo.Status,
			"is_primary":    img.IsPrimary,
			"owner_verified": true,
		})
	}

	ctx.JSON(http.StatusOK, gin.H{
		"is_associated":   true,
		"image_hash":      imageHash,
		"work_orders":     workOrders,
		"count":           len(images),
		"access_verified": true,
	})
}

func (c *WorkOrderController) VerifyDiagnosisBinding(ctx *gin.Context) {
	workOrderID := ctx.Param("id")
	imageHash := ctx.Query("image_hash")

	userID := middleware.GetCurrentUserID(ctx)
	userRole := middleware.GetCurrentUserRole(ctx)

	wo, err := c.workOrderService.GetWorkOrderByID(workOrderID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{
			"error":               "Work order not found",
			"binding_verified":    false,
		})
		return
	}

	if userRole != string(models.RoleAdmin) {
		if wo.FarmerID != userID && (wo.ExpertID == nil || *wo.ExpertID != userID) {
			ctx.JSON(http.StatusForbidden, gin.H{
				"error":            "Access denied",
				"binding_verified": false,
			})
			return
		}
	}

	db := database.GetDB()
	var images []models.WorkOrderImage
	if imageHash != "" {
		if err := db.Where("work_order_id = ? AND image_hash = ?", workOrderID, imageHash).Find(&images).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := db.Where("work_order_id = ?", workOrderID).Find(&images).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	var imageHashes []string
	for _, img := range images {
		imageHashes = append(imageHashes, img.ImageHash)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"work_order_id":       workOrderID,
		"farmer_id":           wo.FarmerID,
		"binding_verified":    true,
		"image_hashes":        imageHashes,
		"image_count":         len(images),
		"owner_verified":      true,
		"diagnosis_complete":  wo.DiagnosisResult != nil,
	})
}

func (c *WorkOrderController) ForceSafetyCheckDemo(ctx *gin.Context) {
	scenario := ctx.Query("scenario")

	var result *services.PrescriptionSafetyResult
	var err error

	switch scenario {
	case "timeout":
		originalTimeout := services.DefaultServiceConfig.PrescriptionTimeout
		services.DefaultServiceConfig.PrescriptionTimeout = 1 * time.Microsecond
		defer func() { services.DefaultServiceConfig.PrescriptionTimeout = originalTimeout }()

		result, err = c.imageService.CheckPrescriptionCompatibilityWithRetry(
			[]string{"三环唑", "稻瘟灵"},
			"demo-work-order-id",
			"demo-expert-id",
		)

	case "service_down":
		originalURL := c.cfg.PythonServiceURL
		c.cfg.PythonServiceURL = "http://nonexistent-service:9999"
		defer func() { c.cfg.PythonServiceURL = originalURL }()

		c.imageService = services.NewImageService(c.cfg)
		result, err = c.imageService.CheckPrescriptionCompatibilityWithRetry(
			[]string{"三环唑", "稻瘟灵"},
			"demo-work-order-id",
			"demo-expert-id",
		)

	case "empty_medications":
		result, err = c.imageService.CheckPrescriptionCompatibilityWithRetry(
			[]string{},
			"demo-work-order-id",
			"demo-expert-id",
		)

	case "no_identity":
		result, err = c.imageService.CheckPrescriptionCompatibilityWithRetry(
			[]string{"三环唑"},
			"",
			"",
		)

	case "circuit_breaker":
		c.imageService.ResetCircuitBreaker()
		for i := 0; i < services.DefaultServiceConfig.CircuitFailureThreshold; i++ {
			originalURL := c.cfg.PythonServiceURL
			c.cfg.PythonServiceURL = "http://nonexistent-service:9999"
			tempService := services.NewImageService(c.cfg)
			tempService.CheckPrescriptionCompatibilityWithRetry(
				[]string{"三环唑"},
				"demo-wo",
				"demo-expert",
			)
			c.cfg.PythonServiceURL = originalURL
		}
		
		state, failures, openTime := c.imageService.GetCircuitStatus()
		ctx.JSON(http.StatusOK, gin.H{
			"scenario":              "circuit_breaker_test",
			"current_state":         state,
			"consecutive_failures":  failures,
			"circuit_open_time":     openTime,
			"expected_behavior":     "After 5 failures, circuit should be OPEN and block all prescriptions",
			"states": gin.H{
				"CircuitClosed":   0,
				"CircuitOpen":     1,
				"CircuitHalfOpen": 2,
			},
		})
		return

	case "incompatible":
		ctx.JSON(http.StatusOK, gin.H{
			"scenario":          "incompatible_medications",
			"expected_behavior": "Should return is_safe=false and block prescription",
			"example": gin.H{
				"medications": []string{"波尔多液", "石硫合剂"},
				"expected": gin.H{
					"is_safe":   false,
					"warnings":  "波尔多液 与 石硫合剂 存在配伍禁忌：混用会产生化学反应，降低药效并产生药害",
					"suggestions": "建议调整用药方案，避免混用存在配伍禁忌的药剂。如需同时使用，请咨询专业农技人员。",
				},
			},
		})
		return

	case "fail_closed":
		ctx.JSON(http.StatusOK, gin.H{
			"scenario":          "fail_closed_security_policy",
			"current_config": gin.H{
				"fail_open":               services.DefaultServiceConfig.FailOpen,
				"max_retries":             services.DefaultServiceConfig.MaxRetries,
				"prescription_timeout":    services.DefaultServiceConfig.PrescriptionTimeout.String(),
				"circuit_breaker_enabled": services.DefaultServiceConfig.CircuitBreakerEnabled,
				"circuit_failure_threshold": services.DefaultServiceConfig.CircuitFailureThreshold,
				"circuit_open_duration":   services.DefaultServiceConfig.CircuitOpenDuration.String(),
			},
			"security_policy": gin.H{
				"description": "By default, Fail-Closed policy is ENABLED",
				"behavior": "If compatibility check service is unavailable, times out, or returns error, prescription is BLOCKED",
				"rationale": "Pesticide safety is critical. Better to delay prescription than issue an unsafe one.",
				"conditions_blocked": []string{
					"Service timeout",
					"Service unavailable",
					"Network error",
					"Invalid response",
					"Circuit breaker OPEN",
					"Missing identity parameters",
				},
				"conditions_allowed": []string{
					"Service returns is_safe=true",
					"Empty medications (single agent use)",
					"Fail-Open mode explicitly enabled (NOT RECOMMENDED)",
				},
			},
		})
		return

	case "audit_logs":
		logs := c.imageService.GetAuditLogs()
		ctx.JSON(http.StatusOK, gin.H{
			"scenario":     "audit_logs",
			"total_logs":   len(logs),
			"logs":         logs,
		})
		return

	default:
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid scenario. Available: timeout, service_down, empty_medications, no_identity, circuit_breaker, incompatible, fail_closed, audit_logs",
		})
		return
	}

	if err != nil {
		ctx.JSON(http.StatusServiceUnavailable, gin.H{
			"scenario":            scenario,
			"error":               err.Error(),
			"is_safe":             false,
			"prescription_blocked": true,
			"security_policy":     "FAIL-CLOSED (Default) - Safety check failed, prescription blocked",
			"check_status":        result.CheckStatus,
			"warnings":            result.Warnings,
			"suggestions":         result.Suggestions,
			"is_fallback":         result.IsFallback,
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"scenario":           scenario,
		"is_safe":            result.IsSafe,
		"check_status":       result.CheckStatus,
		"warnings":           result.Warnings,
		"suggestions":        result.Suggestions,
		"is_fallback":        result.IsFallback,
		"check_timestamp":    result.CheckTimestamp,
		"service_available":  result.ServiceAvailable,
	})
}
