package services

import (
	"math"
	"time"

	"github.com/jinzhu/gorm"

	"agriculture-platform/database"
	"agriculture-platform/models"
)

type WorkOrderService struct {
	db *gorm.DB
}

func NewWorkOrderService() *WorkOrderService {
	return &WorkOrderService{
		db: database.GetDB(),
	}
}

func (s *WorkOrderService) CreateWorkOrder(wo *models.WorkOrder, farmerID string) (*models.WorkOrder, error) {
	wo.FarmerID = farmerID
	wo.Status = models.StatusPending
	wo.CreatedAt = time.Now()
	wo.UpdatedAt = time.Now()

	if err := s.db.Create(wo).Error; err != nil {
		return nil, err
	}

	s.createStatusTransition(wo.ID, "", models.StatusPending, farmerID, "工单创建")

	return wo, nil
}

func (s *WorkOrderService) GetWorkOrderByID(workOrderID string) (*models.WorkOrder, error) {
	var wo models.WorkOrder
	if err := s.db.Preload("Farmer").Preload("Expert").Preload("DiagnosisResult").Preload("Prescription").Preload("Feedback").First(&wo, "id = ?", workOrderID).Error; err != nil {
		return nil, err
	}

	var images []models.WorkOrderImage
	s.db.Where("work_order_id = ?", wo.ID).Find(&images)

	return &wo, nil
}

func (s *WorkOrderService) GetWorkOrdersByFarmer(farmerID string, status string, page, pageSize int) ([]models.WorkOrder, int64, error) {
	var workOrders []models.WorkOrder
	var total int64

	query := s.db.Model(&models.WorkOrder{}).Where("farmer_id = ?", farmerID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Preload("Expert").Preload("DiagnosisResult").Find(&workOrders).Error; err != nil {
		return nil, 0, err
	}

	return workOrders, total, nil
}

func (s *WorkOrderService) GetWorkOrdersByExpert(expertID string, status string, page, pageSize int) ([]models.WorkOrder, int64, error) {
	var workOrders []models.WorkOrder
	var total int64

	query := s.db.Model(&models.WorkOrder{}).Where("expert_id = ?", expertID)
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Preload("Farmer").Preload("DiagnosisResult").Preload("Prescription").Preload("Feedback").Find(&workOrders).Error; err != nil {
		return nil, 0, err
	}

	return workOrders, total, nil
}

func (s *WorkOrderService) GetPendingWorkOrders(page, pageSize int) ([]models.WorkOrder, int64, error) {
	var workOrders []models.WorkOrder
	var total int64

	query := s.db.Model(&models.WorkOrder{}).Where("status IN ?", []models.WorkOrderStatus{models.StatusPending, models.StatusDiagnosing})

	query.Count(&total)

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("priority DESC, created_at ASC").Preload("Farmer").Preload("DiagnosisResult").Find(&workOrders).Error; err != nil {
		return nil, 0, err
	}

	return workOrders, total, nil
}

func (s *WorkOrderService) UpdateStatus(workOrderID string, newStatus models.WorkOrderStatus, transitedBy string, reason string) error {
	var wo models.WorkOrder
	if err := s.db.First(&wo, "id = ?", workOrderID).Error; err != nil {
		return err
	}

	oldStatus := wo.Status
	wo.Status = newStatus
	wo.UpdatedAt = time.Now()

	if newStatus == models.StatusClosed {
		now := time.Now()
		wo.ClosedAt = &now
	}

	if err := s.db.Save(&wo).Error; err != nil {
		return err
	}

	s.createStatusTransition(workOrderID, oldStatus, newStatus, transitedBy, reason)

	return nil
}

func (s *WorkOrderService) AssignExpert(workOrderID string, expertID string, assignedBy string) error {
	var wo models.WorkOrder
	if err := s.db.First(&wo, "id = ?", workOrderID).Error; err != nil {
		return err
	}

	expertIDPtr := &expertID
	wo.ExpertID = expertIDPtr
	now := time.Now()
	wo.AssignedAt = &now
	wo.Status = models.StatusAssigned
	wo.UpdatedAt = now

	if err := s.db.Save(&wo).Error; err != nil {
		return err
	}

	s.createStatusTransition(workOrderID, models.StatusDiagnosing, models.StatusAssigned, assignedBy, "分配专家")

	return nil
}

func (s *WorkOrderService) SaveDiagnosisResult(workOrderID string, result *models.DiagnosisResult) error {
	var existingResult models.DiagnosisResult
	if err := s.db.Where("work_order_id = ?", workOrderID).First(&existingResult).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			result.WorkOrderID = workOrderID
			result.CreatedAt = time.Now()
			result.UpdatedAt = time.Now()
			return s.db.Create(result).Error
		}
		return err
	}

	existingResult.DiseaseName = result.DiseaseName
	existingResult.DiseaseType = result.DiseaseType
	existingResult.Confidence = result.Confidence
	existingResult.Symptoms = result.Symptoms
	existingResult.Causes = result.Causes
	existingResult.RecommendedActions = result.RecommendedActions
	existingResult.Severity = result.Severity
	existingResult.SimilarCases = result.SimilarCases
	existingResult.UpdatedAt = time.Now()

	return s.db.Save(&existingResult).Error
}

func (s *WorkOrderService) CreatePrescription(workOrderID string, prescription *models.Prescription, expertID string) error {
	prescription.WorkOrderID = workOrderID
	prescription.ExpertID = expertID
	prescription.CreatedAt = time.Now()
	prescription.UpdatedAt = time.Now()

	if err := s.db.Create(prescription).Error; err != nil {
		return err
	}

	return s.UpdateStatus(workOrderID, models.StatusPrescribed, expertID, "开具处方")
}

func (s *WorkOrderService) CreateFeedback(workOrderID string, feedback *models.Feedback, farmerID string) error {
	feedback.WorkOrderID = workOrderID
	feedback.FarmerID = farmerID
	feedback.CreatedAt = time.Now()
	feedback.UpdatedAt = time.Now()

	if err := s.db.Create(feedback).Error; err != nil {
		return err
	}

	var wo models.WorkOrder
	if err := s.db.First(&wo, "id = ?", workOrderID).Error; err != nil {
		return err
	}

	if wo.ExpertID != nil {
		userService := NewUserService()
		userService.UpdateExpertRating(*wo.ExpertID, feedback.Rating)
	}

	return s.UpdateStatus(workOrderID, models.StatusClosed, farmerID, "用户提交反馈，工单关闭")
}

func (s *WorkOrderService) AddWorkOrderImage(image *models.WorkOrderImage) error {
	image.CreatedAt = time.Now()
	return s.db.Create(image).Error
}

func (s *WorkOrderService) GetWorkOrderImages(workOrderID string) ([]models.WorkOrderImage, error) {
	var images []models.WorkOrderImage
	if err := s.db.Where("work_order_id = ?", workOrderID).Find(&images).Error; err != nil {
		return nil, err
	}
	return images, nil
}

func (s *WorkOrderService) createStatusTransition(workOrderID string, fromStatus models.WorkOrderStatus, toStatus models.WorkOrderStatus, transitedBy string, reason string) {
	transition := &models.StatusTransition{
		WorkOrderID: workOrderID,
		FromStatus:  fromStatus,
		ToStatus:    toStatus,
		TransitedBy: transitedBy,
		Reason:      reason,
		CreatedAt:   time.Now(),
	}
	s.db.Create(transition)
}

func (s *WorkOrderService) FindNearestExpert(lat, lng float64, specialization string) (*models.ExpertProfile, error) {
	var experts []models.ExpertProfile
	query := s.db.Where("is_available = ?", true)

	if specialization != "" {
		query = query.Where("specialization LIKE ?", "%"+specialization+"%")
	}

	if err := query.Find(&experts).Error; err != nil {
		return nil, err
	}

	if len(experts) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var bestMatch *models.ExpertProfile
	minDistance := math.Inf(1)

	for i := range experts {
		distance := calculateDistance(lat, lng, experts[i].Latitude, experts[i].Longitude)
		if distance < minDistance {
			minDistance = distance
			bestMatch = &experts[i]
		}
	}

	return bestMatch, nil
}

func calculateDistance(lat1, lng1, lat2, lng2 float64) float64 {
	earthRadius := 6371.0

	latRad1 := lat1 * math.Pi / 180
	lngRad1 := lng1 * math.Pi / 180
	latRad2 := lat2 * math.Pi / 180
	lngRad2 := lng2 * math.Pi / 180

	dLat := latRad2 - latRad1
	dLng := lngRad2 - lngRad1

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(latRad1)*math.Cos(latRad2)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadius * c
}

func (s *WorkOrderService) SyncOfflineWorkOrders(farmerID string, offlineOrders []models.WorkOrder) error {
	for _, order := range offlineOrders {
		order.FarmerID = farmerID
		order.IsOfflineCreated = true
		order.OfflineSyncStatus = "synced"
		order.Status = models.StatusPending
		order.CreatedAt = time.Now()
		order.UpdatedAt = time.Now()

		if err := s.db.Create(&order).Error; err != nil {
			return err
		}

		s.createStatusTransition(order.ID, "", models.StatusPending, farmerID, "离线工单同步")
	}
	return nil
}
