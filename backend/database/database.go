package database

import (
	"log"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"

	"agriculture-platform/config"
	"agriculture-platform/models"
)

var DB *gorm.DB

func InitDB(cfg *config.Config) {
	var err error

	DB, err = gorm.Open("sqlite3", "./agriculture.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Database connected successfully")

	DB.AutoMigrate(
		&models.User{},
		&models.FarmerProfile{},
		&models.ExpertProfile{},
		&models.WorkOrder{},
		&models.DiagnosisResult{},
		&models.Prescription{},
		&models.Feedback{},
		&models.WorkOrderImage{},
		&models.StatusTransition{},
		&models.Message{},
		&models.Notification{},
	)

	log.Println("Database migration completed")

	seedData()
}

func seedData() {
	var count int
	DB.Model(&models.User{}).Count(&count)
	if count > 0 {
		return
	}

	log.Println("Seeding initial data...")

	expert := &models.User{
		Username: "expert1",
		FullName: "张农技",
		Phone:    "13800138001",
		Role:     models.RoleExpert,
	}
	expert.SetPassword("password123")
	DB.Create(expert)

	expertProfile := &models.ExpertProfile{
		UserID:         expert.ID,
		Specialization: "水稻病虫害防治,蔬菜种植",
		Location:       "湖南省长沙市",
		Latitude:       28.2282,
		Longitude:      112.9388,
		Rating:         4.8,
		TotalReviews:   56,
		ExperienceYears: 15,
		Certifications: "高级农艺师,植保专家",
		IsAvailable:    true,
	}
	DB.Create(expertProfile)

	expert2 := &models.User{
		Username: "expert2",
		FullName: "李植保",
		Phone:    "13800138002",
		Role:     models.RoleExpert,
	}
	expert2.SetPassword("password123")
	DB.Create(expert2)

	expertProfile2 := &models.ExpertProfile{
		UserID:         expert2.ID,
		Specialization: "果树病虫害,土壤改良",
		Location:       "湖北省武汉市",
		Latitude:       30.5928,
		Longitude:      114.3055,
		Rating:         4.9,
		TotalReviews:   89,
		ExperienceYears: 20,
		Certifications: "农业技术推广研究员",
		IsAvailable:    true,
	}
	DB.Create(expertProfile2)

	farmer := &models.User{
		Username: "farmer1",
		FullName: "王农户",
		Phone:    "13900139001",
		Role:     models.RoleFarmer,
	}
	farmer.SetPassword("password123")
	DB.Create(farmer)

	farmerProfile := &models.FarmerProfile{
		UserID:    farmer.ID,
		Location:  "湖南省岳阳市",
		Latitude:  29.3763,
		Longitude: 113.1339,
		FarmSize:  50.5,
		Crops:     "水稻,蔬菜",
	}
	DB.Create(farmerProfile)

	log.Println("Initial data seeded successfully")
}

func GetDB() *gorm.DB {
	return DB
}
