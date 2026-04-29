package models

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRole string

const (
	RoleFarmer  UserRole = "farmer"
	RoleExpert  UserRole = "expert"
	RoleAdmin   UserRole = "admin"
)

type User struct {
	ID           string     `gorm:"primary_key;type:varchar(36)" json:"id"`
	Username     string     `gorm:"unique_index;type:varchar(50)" json:"username"`
	PasswordHash string     `gorm:"type:varchar(255)" json:"-"`
	FullName     string     `gorm:"type:varchar(100)" json:"full_name"`
	Phone        string     `gorm:"type:varchar(20)" json:"phone"`
	Role         UserRole   `gorm:"type:varchar(20)" json:"role"`
	Avatar       string     `gorm:"type:varchar(255)" json:"avatar"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	FarmerProfile *FarmerProfile `gorm:"foreignkey:UserID" json:"farmer_profile,omitempty"`
	ExpertProfile *ExpertProfile `gorm:"foreignkey:UserID" json:"expert_profile,omitempty"`
}

type FarmerProfile struct {
	ID          string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	UserID      string    `gorm:"type:varchar(36);index" json:"user_id"`
	Location    string    `gorm:"type:varchar(255)" json:"location"`
	Latitude    float64   `json:"latitude"`
	Longitude   float64   `json:"longitude"`
	FarmSize    float64   `json:"farm_size"`
	Crops       string    `gorm:"type:text" json:"crops"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ExpertProfile struct {
	ID              string    `gorm:"primary_key;type:varchar(36)" json:"id"`
	UserID          string    `gorm:"type:varchar(36);index" json:"user_id"`
	Specialization  string    `gorm:"type:varchar(255)" json:"specialization"`
	Location        string    `gorm:"type:varchar(255)" json:"location"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	Rating          float64   `gorm:"default:0" json:"rating"`
	TotalReviews    int       `gorm:"default:0" json:"total_reviews"`
	ExperienceYears int       `json:"experience_years"`
	Certifications  string    `gorm:"type:text" json:"certifications"`
	IsAvailable     bool      `gorm:"default:true" json:"is_available"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (u *User) BeforeCreate() (err error) {
	u.ID = uuid.New().String()
	return nil
}

func (fp *FarmerProfile) BeforeCreate() (err error) {
	fp.ID = uuid.New().String()
	return nil
}

func (ep *ExpertProfile) BeforeCreate() (err error) {
	ep.ID = uuid.New().String()
	return nil
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
