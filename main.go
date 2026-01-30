package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory-ledger/src/config"
	"inventory-ledger/src/handlers"
	"inventory-ledger/src/models"
	"inventory-ledger/src/repositories"
	"inventory-ledger/src/routes"
	"inventory-ledger/src/services"
)

func main() {
	db := config.InitDB()

	db.AutoMigrate(
		&models.Organization{},
		&models.Item{},
		&models.Inventory{},
		&models.InventoryHistory{},
	)

	// Insert sample data jika kosong
	if err := seedSampleData(db); err != nil {
		log.Printf("Failed to seed sample data: %v", err)
	}

	// Initialize repository
	repo := &repositories.InventoryRepository{DB: db}

	// Initialize service
	service := &services.InventoryService{
		DB:   db,
		Repo: repo,
	}

	// Initialize handler
	handler := &handlers.InventoryHandler{
		Service: service,
	}

	// Setup router dengan recovery middleware
	router := gin.Default()

	api := router.Group("/api/v1")
	routes.RegisterInventoryRoutes(api.Group("/inventory"), handler)

	// Start server
	port := ":8080"

	if err := router.Run(port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}

func seedSampleData(db *gorm.DB) error {
	var orgCount int64
	db.Model(&models.Organization{}).Count(&orgCount)

	if orgCount == 0 {
		log.Println("🌱 Seeding sample organizations...")

		organizations := []models.Organization{
			{ID: mustParseUUID("b159a190-e72f-4295-853c-ddbbe19fa6f6"), Name: "Main Warehouse", Code: "WH-MAIN"},
			{ID: mustParseUUID("2003eacc-5f39-4f3d-94d7-6e01c1bebd6a"), Name: "Branch Office 1", Code: "BR-001"},
			{ID: mustParseUUID("9cf2bfa5-29b7-4be4-a9cc-969e567f8fe3"), Name: "Branch Office 2", Code: "BR-002"},
			{ID: mustParseUUID("545dc82f-3ea6-4355-be7c-18821ad8940c"), Name: "Retail Store", Code: "RT-001"},
		}

		for _, org := range organizations {
			if err := db.FirstOrCreate(&org, "id = ?", org.ID).Error; err != nil {
				return err
			}
		}
		log.Printf("✅ Seeded %d organizations", len(organizations))
	}

	var itemCount int64
	db.Model(&models.Item{}).Count(&itemCount)

	if itemCount == 0 {
		log.Println("🌱 Seeding sample items...")

		items := []models.Item{
			{Code: "ITEM-001", Name: "Laptop Dell XPS 13", Unit: "unit"},
			{Code: "ITEM-002", Name: "Mouse Wireless Logitech", Unit: "pcs"},
			{Code: "ITEM-003", Name: "Monitor 24 inch", Unit: "unit"},
			{Code: "ITEM-004", Name: "Keyboard Mechanical", Unit: "pcs"},
		}

		for _, item := range items {
			if err := db.FirstOrCreate(&item, "code = ?", item.Code).Error; err != nil {
				return err
			}
		}
		log.Printf("✅ Seeded %d items", len(items))
	}

	return nil
}

func mustParseUUID(s string) uuid.UUID {
	id, err := uuid.Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}
