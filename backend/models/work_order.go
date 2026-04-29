package models

import (
	"time"

	"github.com/google/uuid"
)

type WorkOrderStatus string

const (
	StatusPending       WorkOrderStatus = "pending"
	StatusDiagnosing    WorkOrderStatus = "diagnosing"
	StatusAssigned      WorkOrderStatus = "assigned"
	StatusConsulting    WorkOrderStatus = "consulting"
	StatusPrescribed    WorkOrderStatus = "prescribed"
	StatusConfirmed     WorkOrderStatus = "confirmed"
	StatusFollowUp      WorkOrderStatus = "follow_up"
	StatusClosed        WorkOrderStatus = "closed"
	StatusCancelled     WorkOrderStatus = "cancelled"
)

type WorkOrder struct {
	ID                 string          `gorm:"primary_key;type:varchar(36)" json:"id"`
	FarmerID           string          `gorm:"type:varchar(36);index" json:"farmer_id"`
	ExpertID           *string         `gorm:"type:varchar(36);index" json:"expert_id,omitempty"`
	Title              string          `gorm:"type:varchar(200)" json:"title"`
	Description        string          `gorm:"type:text" json:"description"`
	CropType           string          `gorm:"type:varchar(100)" json:"crop_type"`
	Location           string          `gorm:"type:varchar(255)" json:"location"`
	Latitude           float64         `json:"latitude"`
	Longitude          float64         `json:"longitude"`
	Status             WorkOrderStatus `gorm:"type:varchar(20);index" json:"status"`
	Priority           int             `gorm:"default:1" json:"priority"`
	AIConfidence       float64         `json:"ai_confidence"`
	DiagnosisResult    *DiagnosisResult `gorm:"foreignkey:WorkOrderID" json:"diagnosis_result,omitempty"`
	Prescription       *Prescription   `gorm:"foreignkey:WorkOrderID" json:"prescription,omitempty"`
	Feedback           *Feedback       `gorm:"foreignkey:WorkOrderID" json:"feedback,omitempty"`
	IsOfflineCreated   bool            `gorm:"default:false" json:"is_offline_created"`
	OfflineSyncStatus  string          `gorm:"type:varchar(20);default:'synced'" json:"offline_sync_status"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
	AssignedAt         *time.Time      `json:"assigned_at,omitempty"`
	ClosedAt           *time.Time      `json:"closed_at,omitempty"`

	Farmer *User `gorm:"foreignkey:FarmerID" json:"farmer,omitempty"`
	Expert *User `gorm:"foreignkey:ExpertID" json:"expert,omitempty"`
}

type DiagnosisResult struct {
	ID                string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID       string    `gorm:"type:varchar(36);unique_index" json:"work_order_id"`
	DiseaseName       string    `gorm:"type:varchar(200)" json:"disease_name"`
	DiseaseType       string    `gorm:"type:varchar(100)" json:"disease_type"`
	Confidence        float64   `json:"confidence"`
	Symptoms          string    `gorm:"type:text" json:"symptoms"`
	Causes            string    `gorm:"type:text" json:"causes"`
	RecommendedActions string    `gorm:"type:text" json:"recommended_actions"`
	Severity          string    `gorm:"type:varchar(50)" json:"severity"`
	SimilarCases      string    `gorm:"type:text" json:"similar_cases"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Prescription struct {
	ID               string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID      string    `gorm:"type:varchar(36);unique_index" json:"work_order_id"`
	ExpertID         string    `gorm:"type:varchar(36)" json:"expert_id"`
	Diagnosis        string    `gorm:"type:text" json:"diagnosis"`
	TreatmentPlan    string    `gorm:"type:text" json:"treatment_plan"`
	Medications      string    `gorm:"type:text" json:"medications"`
	Dosage           string    `gorm:"type:text" json:"dosage"`
	ApplicationMethod string    `gorm:"type:text" json:"application_method"`
	PreventionTips   string    `gorm:"type:text" json:"prevention_tips"`
	FollowUpDate     *time.Time `json:"follow_up_date"`
	Notes            string    `gorm:"type:text" json:"notes"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	Expert *User `gorm:"foreignkey:ExpertID" json:"expert,omitempty"`
}

type Feedback struct {
	ID             string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID    string    `gorm:"type:varchar(36);unique_index" json:"work_order_id"`
	FarmerID       string    `gorm:"type:varchar(36)" json:"farmer_id"`
	Rating         int       `gorm:"type:int" json:"rating"`
	Effectiveness  string    `gorm:"type:text" json:"effectiveness"`
	Comments       string    `gorm:"type:text" json:"comments"`
	Improvements   string    `gorm:"type:text" json:"improvements"`
	IsSolved       bool      `gorm:"default:false" json:"is_solved"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type WorkOrderImage struct {
	ID           string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID  string    `gorm:"type:varchar(36);index" json:"work_order_id"`
	ImageURL     string    `gorm:"type:varchar(500)" json:"image_url"`
	ImageHash    string    `gorm:"type:varchar(64);index" json:"image_hash"`
	IsPrimary    bool      `gorm:"default:false" json:"is_primary"`
	CaptureTime  *time.Time `json:"capture_time"`
	Location     string    `gorm:"type:varchar(255)" json:"location"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	CreatedAt    time.Time `json:"created_at"`
}

type StatusTransition struct {
	ID          string          `gorm:"primary_key;type:varchar(36)" json:"id"`
	WorkOrderID string          `gorm:"type:varchar(36);index" json:"work_order_id"`
	FromStatus  WorkOrderStatus `gorm:"type:varchar(20)" json:"from_status"`
	ToStatus    WorkOrderStatus `gorm:"type:varchar(20)" json:"to_status"`
	TransitedBy string          `gorm:"type:varchar(36)" json:"transited_by"`
	Reason      string          `gorm:"type:text" json:"reason"`
	CreatedAt   time.Time       `json:"created_at"`
}

func (wo *WorkOrder) BeforeCreate() (err error) {
	wo.ID = uuid.New().String()
	return nil
}

func (dr *DiagnosisResult) BeforeCreate() (err error) {
	dr.ID = uuid.New().String()
	return nil
}

func (p *Prescription) BeforeCreate() (err error) {
	p.ID = uuid.New().String()
	return nil
}

func (f *Feedback) BeforeCreate() (err error) {
	f.ID = uuid.New().String()
	return nil
}

func (woi *WorkOrderImage) BeforeCreate() (err error) {
	woi.ID = uuid.New().String()
	return nil
}

func (st *StatusTransition) BeforeCreate() (err error) {
	st.ID = uuid.New().String()
	return nil
}
