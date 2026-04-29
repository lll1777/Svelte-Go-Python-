package services

import (
	"errors"

	"github.com/jinzhu/gorm"

	"agriculture-platform/database"
	"agriculture-platform/models"
)

type UserService struct {
	db *gorm.DB
}

func NewUserService() *UserService {
	return &UserService{
		db: database.GetDB(),
	}
}

func (s *UserService) RegisterFarmer(username, password, fullName, phone string, profile *models.FarmerProfile) (*models.User, error) {
	var existingUser models.User
	if err := s.db.Where("username = ?", username).First(&existingUser).Error; err == nil {
		return nil, errors.New("username already exists")
	}

	user := &models.User{
		Username: username,
		FullName: fullName,
		Phone:    phone,
		Role:     models.RoleFarmer,
	}

	if err := user.SetPassword(password); err != nil {
		return nil, err
	}

	if err := s.db.Create(user).Error; err != nil {
		return nil, err
	}

	if profile != nil {
		profile.UserID = user.ID
		if err := s.db.Create(profile).Error; err != nil {
			return nil, err
		}
		user.FarmerProfile = profile
	}

	return user, nil
}

func (s *UserService) Login(username, password string) (*models.User, error) {
	var user models.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if !user.CheckPassword(password) {
		return nil, errors.New("invalid credentials")
	}

	if user.Role == models.RoleFarmer {
		var profile models.FarmerProfile
		s.db.Where("user_id = ?", user.ID).First(&profile)
		user.FarmerProfile = &profile
	} else if user.Role == models.RoleExpert {
		var profile models.ExpertProfile
		s.db.Where("user_id = ?", user.ID).First(&profile)
		user.ExpertProfile = &profile
	}

	return &user, nil
}

func (s *UserService) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	if err := s.db.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}

	if user.Role == models.RoleFarmer {
		var profile models.FarmerProfile
		s.db.Where("user_id = ?", user.ID).First(&profile)
		user.FarmerProfile = &profile
	} else if user.Role == models.RoleExpert {
		var profile models.ExpertProfile
		s.db.Where("user_id = ?", user.ID).First(&profile)
		user.ExpertProfile = &profile
	}

	return &user, nil
}

func (s *UserService) UpdateFarmerProfile(userID string, profile *models.FarmerProfile) error {
	var existingProfile models.FarmerProfile
	if err := s.db.Where("user_id = ?", userID).First(&existingProfile).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			profile.UserID = userID
			return s.db.Create(profile).Error
		}
		return err
	}

	existingProfile.Location = profile.Location
	existingProfile.Latitude = profile.Latitude
	existingProfile.Longitude = profile.Longitude
	existingProfile.FarmSize = profile.FarmSize
	existingProfile.Crops = profile.Crops

	return s.db.Save(&existingProfile).Error
}

func (s *UserService) GetExpertByID(expertID string) (*models.ExpertProfile, error) {
	var profile models.ExpertProfile
	if err := s.db.Where("user_id = ?", expertID).First(&profile).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *UserService) UpdateExpertProfile(userID string, profile *models.ExpertProfile) error {
	var existingProfile models.ExpertProfile
	if err := s.db.Where("user_id = ?", userID).First(&existingProfile).Error; err != nil {
		if gorm.IsRecordNotFoundError(err) {
			profile.UserID = userID
			return s.db.Create(profile).Error
		}
		return err
	}

	existingProfile.Specialization = profile.Specialization
	existingProfile.Location = profile.Location
	existingProfile.Latitude = profile.Latitude
	existingProfile.Longitude = profile.Longitude
	existingProfile.ExperienceYears = profile.ExperienceYears
	existingProfile.Certifications = profile.Certifications
	existingProfile.IsAvailable = profile.IsAvailable

	return s.db.Save(&existingProfile).Error
}

func (s *UserService) UpdateExpertRating(expertID string, newRating int) error {
	var profile models.ExpertProfile
	if err := s.db.Where("user_id = ?", expertID).First(&profile).Error; err != nil {
		return err
	}

	totalReviews := float64(profile.TotalReviews)
	currentRating := profile.Rating

	newTotal := totalReviews + 1
	newRatingAvg := (currentRating*totalReviews + float64(newRating)) / newTotal

	profile.Rating = newRatingAvg
	profile.TotalReviews = int(newTotal)

	return s.db.Save(&profile).Error
}
