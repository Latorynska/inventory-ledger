package services_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"inventory-ledger/src/models"
	"inventory-ledger/src/repositories"
	"inventory-ledger/src/services"
)

var (
	testDB      *gorm.DB
	testOrg1ID  uuid.UUID
	testOrg2ID  uuid.UUID
	testItemID  uint = 1
	testService *services.InventoryService
)

func setupTestDB() *gorm.DB {
	dsn := "host=localhost user=postgres password=Lotarynska0906 dbname=inventory_test port=5432 sslmode=disable"

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logger.Info,
			IgnoreRecordNotFoundError: false,
			ParameterizedQueries:      true,
			Colorful:                  true,
		},
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: newLogger,
	})
	if err != nil {
		panic("failed to connect database")
	}

	// Auto migrate
	db.AutoMigrate(
		&models.Inventory{},
		&models.InventoryHistory{},
		&models.Organization{},
		&models.Item{},
	)

	return db
}

func cleanupTestDB(db *gorm.DB) {
	db.Exec("TRUNCATE inventories, inventory_histories, organizations, items RESTART IDENTITY CASCADE")
}

func setupTestData(db *gorm.DB) {
	// Create test organizations
	testOrg1ID = uuid.New()
	testOrg2ID = uuid.New()

	orgs := []models.Organization{
		{ID: testOrg1ID, Name: "Test Org 1", Code: "ORG001"},
		{ID: testOrg2ID, Name: "Test Org 2", Code: "ORG002"},
	}

	for _, org := range orgs {
		db.Create(&org)
	}

	// Create test item
	item := models.Item{
		ID:   testItemID,
		Code: "ITEM001",
		Name: "Test Item",
		Unit: "pcs",
	}
	db.Create(&item)
}

func TestMain(m *testing.M) {
	fmt.Println("Setting up test database...")
	testDB = setupTestDB()

	// Run cleanup before tests
	cleanupTestDB(testDB)
	setupTestData(testDB)

	// Create service
	repo := &repositories.InventoryRepository{DB: testDB}
	testService = &services.InventoryService{
		DB:   testDB,
		Repo: repo,
	}

	// Run tests
	code := m.Run()

	// Cleanup after tests
	cleanupTestDB(testDB)

	os.Exit(code)
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func assertError(t *testing.T, err error, expectedMsg string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error but got none")
	}
	if expectedMsg != "" && err.Error() != expectedMsg {
		t.Fatalf("expected error: %s, got: %v", expectedMsg, err)
	}
}

func assertEqual(t *testing.T, expected, actual interface{}, msg ...string) {
	t.Helper()
	if expected != actual {
		message := ""
		if len(msg) > 0 {
			message = msg[0]
		}
		t.Errorf("%sexpected %v, got %v", message, expected, actual)
	}
}

// ============ TEST SCENARIO 1: BASIC TRANSACTION FLOW ============
func TestBasicTransactionFlow(t *testing.T) {
	t.Run("SC1: Create penerimaan and verify balance", func(t *testing.T) {
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			Amount:         100,
			Type:           "penerimaan",
			ChangedBy:      "user1",
		}

		inv, err := testService.CreateTransaction(req)
		assertNoError(t, err)
		assertEqual(t, 100, inv.Balance)

		// Verify current balance
		balance, err := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertNoError(t, err)
		assertEqual(t, 100, balance)

		// Verify historical balance
		historical, err := testService.GetBalanceAt(testOrg1ID, testItemID,
			time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC))
		assertNoError(t, err)
		assertEqual(t, 100, historical)
	})

	t.Run("SC2: Create pemakaian and verify balance", func(t *testing.T) {
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
			Amount:         -30,
			Type:           "pemakaian",
			ChangedBy:      "user1",
		}

		inv, err := testService.CreateTransaction(req)
		assertNoError(t, err)
		assertEqual(t, 70, inv.Balance) // 100 - 30 = 70

		current, err := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertNoError(t, err)
		assertEqual(t, 70, current)
	})

	t.Run("SC3: Create stok_awal for new org", func(t *testing.T) {
		newOrgID := uuid.New()
		testDB.Create(&models.Organization{
			ID:   newOrgID,
			Name: "New Org",
			Code: "ORG003",
		})

		req := services.CreateTransactionRequest{
			OrganizationID: newOrgID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 1, 1, 9, 0, 0, 0, time.UTC),
			Amount:         50,
			Type:           "stok_awal",
			ChangedBy:      "admin",
		}

		inv, err := testService.CreateTransaction(req)
		assertNoError(t, err)
		assertEqual(t, 50, inv.Balance)

		// Try to create second stok_awal - should fail
		req2 := services.CreateTransactionRequest{
			OrganizationID: newOrgID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 1, 2, 10, 0, 0, 0, time.UTC),
			Amount:         20,
			Type:           "stok_awal",
			ChangedBy:      "admin",
		}

		_, err = testService.CreateTransaction(req2)
		assertError(t, err, "stok awal already exists for this item")
	})
}

// ============ TEST SCENARIO 2: UPDATE TRANSACTION ============
func TestUpdateTransaction(t *testing.T) {
	// Setup: Create initial transactions
	req1 := services.CreateTransactionRequest{
		OrganizationID: testOrg2ID,
		ItemID:         testItemID,
		TxnDate:        time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
		Amount:         200,
		Type:           "penerimaan",
		ChangedBy:      "user1",
	}
	inv1, _ := testService.CreateTransaction(req1)

	req2 := services.CreateTransactionRequest{
		OrganizationID: testOrg2ID,
		ItemID:         testItemID,
		TxnDate:        time.Date(2024, 2, 2, 10, 0, 0, 0, time.UTC),
		Amount:         -50,
		Type:           "pemakaian",
		ChangedBy:      "user1",
	}
	inv2, _ := testService.CreateTransaction(req2)

	t.Run("SC4: Update first transaction amount", func(t *testing.T) {
		// Current balance: 200 - 50 = 150
		currentBefore, _ := testService.GetCurrentBalance(testOrg2ID, testItemID)
		assertEqual(t, 150, currentBefore)

		// Update first transaction from 200 to 250
		updateReq := services.UpdateTransactionRequest{
			InventoryID: inv1.ID,
			TxnDate:     time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC),
			Amount:      250, // Increase by 50
			ChangedBy:   "user2",
			Reason:      stringPtr("Correction"),
		}

		err := testService.UpdateTransaction(updateReq)
		assertNoError(t, err)

		// New balance should be: 250 - 50 = 200
		currentAfter, _ := testService.GetCurrentBalance(testOrg2ID, testItemID)
		assertEqual(t, 200, currentAfter)

		// Verify second transaction balance updated
		var updatedInv2 models.Inventory
		testDB.First(&updatedInv2, inv2.ID)
		assertEqual(t, 200, updatedInv2.Balance) // 250 - 50 = 200
	})

	t.Run("SC5: Update transaction date earlier", func(t *testing.T) {
		// Create a transaction
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg2ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 2, 10, 10, 0, 0, 0, time.UTC),
			Amount:         -20,
			Type:           "pemakaian",
			ChangedBy:      "user1",
		}
		inv, _ := testService.CreateTransaction(req)

		// Update to earlier date
		updateReq := services.UpdateTransactionRequest{
			InventoryID: inv.ID,
			TxnDate:     time.Date(2024, 2, 5, 10, 0, 0, 0, time.UTC), // Earlier date
			Amount:      -20,
			ChangedBy:   "user2",
		}

		err := testService.UpdateTransaction(updateReq)
		assertNoError(t, err)

		// Should still have correct balance
		balance, _ := testService.GetCurrentBalance(testOrg2ID, testItemID)
		// Before: 200 (from SC4), after: 200 - 20 = 180
		assertEqual(t, 180, balance)
	})

	t.Run("SC6: Update non-existent transaction", func(t *testing.T) {
		updateReq := services.UpdateTransactionRequest{
			InventoryID: uuid.New(), // Random ID
			TxnDate:     time.Now(),
			Amount:      100,
			ChangedBy:   "user1",
		}

		err := testService.UpdateTransaction(updateReq)
		assertError(t, err, "record not found")
	})
}

// ============ TEST SCENARIO 3: DELETE TRANSACTION ============
func TestDeleteTransaction(t *testing.T) {
	t.Run("SC7: Delete middle transaction", func(t *testing.T) {
		// Create sequence: 100 -> -30 -> -20
		req1 := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
			Amount:         100,
			Type:           "penerimaan",
			ChangedBy:      "user1",
		}
		inv1, _ := testService.CreateTransaction(req1)

		req2 := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 3, 2, 10, 0, 0, 0, time.UTC),
			Amount:         -30,
			Type:           "pemakaian",
			ChangedBy:      "user1",
		}
		inv2, _ := testService.CreateTransaction(req2)

		req3 := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 3, 3, 10, 0, 0, 0, time.UTC),
			Amount:         -20,
			Type:           "pemakaian",
			ChangedBy:      "user1",
		}
		inv3, _ := testService.CreateTransaction(req3)

		// Current balance: 100 - 30 - 20 = 50
		currentBefore, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertEqual(t, 50, currentBefore)

		// Delete middle transaction (-30)
		err := testService.DeleteTransaction(inv2.ID, "admin", stringPtr("Wrong entry"))
		assertNoError(t, err)

		// New balance: 100 - 20 = 80
		currentAfter, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertEqual(t, 80, currentAfter)

		// Verify last transaction balance updated
		var updatedInv3 models.Inventory
		testDB.First(&updatedInv3, inv3.ID)
		assertEqual(t, 80, updatedInv3.Balance) // 100 - 20 = 80

		// Verify first transaction unchanged
		var updatedInv1 models.Inventory
		testDB.First(&updatedInv1, inv1.ID)
		assertEqual(t, 100, updatedInv1.Balance)
	})
}

// ============ TEST SCENARIO 4: MUTATION ============
func TestMutation(t *testing.T) {
	t.Run("SC8: Successful mutation", func(t *testing.T) {
		// Setup source org balance
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 4, 1, 10, 0, 0, 0, time.UTC),
			Amount:         500,
			Type:           "penerimaan",
			ChangedBy:      "user1",
		}
		testService.CreateTransaction(req)

		sourceBefore, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		destBefore, _ := testService.GetCurrentBalance(testOrg2ID, testItemID)

		// Create mutation: transfer 150 from org1 to org2
		mutationReq := services.MutationRequest{
			FromOrganizationID: testOrg1ID,
			ToOrganizationID:   testOrg2ID,
			ItemID:             testItemID,
			Quantity:           150,
			TxnDate:            time.Date(2024, 4, 2, 10, 0, 0, 0, time.UTC),
			ChangedBy:          "admin",
			Reason:             stringPtr("Stock transfer"),
		}

		err := testService.CreateMutation(mutationReq)
		assertNoError(t, err)

		// Verify balances
		sourceAfter, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		destAfter, _ := testService.GetCurrentBalance(testOrg2ID, testItemID)

		assertEqual(t, sourceBefore-150, sourceAfter) // 500 - 150 = 350
		assertEqual(t, destBefore+150, destAfter)     // 0 + 150 = 150
	})

	t.Run("SC9: Mutation with insufficient stock", func(t *testing.T) {
		mutationReq := services.MutationRequest{
			FromOrganizationID: testOrg1ID,
			ToOrganizationID:   testOrg2ID,
			ItemID:             testItemID,
			Quantity:           1000, // More than available
			TxnDate:            time.Date(2024, 4, 3, 10, 0, 0, 0, time.UTC),
			ChangedBy:          "admin",
		}

		err := testService.CreateMutation(mutationReq)
		assertError(t, err, "insufficient stock in source organization")
	})
}

// ============ TEST SCENARIO 5: OPNAME ============
func TestOpname(t *testing.T) {
	t.Run("SC10: Opname with physical > system", func(t *testing.T) {
		// Setup: System shows 100
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Date(2024, 5, 1, 10, 0, 0, 0, time.UTC),
			Amount:         100,
			Type:           "penerimaan",
			ChangedBy:      "user1",
		}
		testService.CreateTransaction(req)

		// Opname finds 120 physical (difference +20)
		opnameReq := services.OpnameRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			PhysicalQty:    120,
			TxnDate:        time.Date(2024, 5, 2, 10, 0, 0, 0, time.UTC),
			ChangedBy:      "auditor",
			Reason:         stringPtr("Monthly stock take"),
		}

		inv, err := testService.CreateOpname(opnameReq)
		assertNoError(t, err)
		assertEqual(t, 120, inv.Balance)
		assertEqual(t, 20, *inv.Difference) // 120 - 100 = +20

		// Verify current balance
		current, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertEqual(t, 120, current)
	})

	t.Run("SC11: Opname with physical < system", func(t *testing.T) {
		// Opname finds 80 physical (difference -40 from previous 120)
		opnameReq := services.OpnameRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			PhysicalQty:    80,
			TxnDate:        time.Date(2024, 5, 3, 10, 0, 0, 0, time.UTC),
			ChangedBy:      "auditor",
		}

		inv, err := testService.CreateOpname(opnameReq)
		assertNoError(t, err)
		assertEqual(t, 80, inv.Balance)
		assertEqual(t, -40, *inv.Difference) // 80 - 120 = -40

		current, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		assertEqual(t, 80, current)
	})
}

// ============ TEST SCENARIO 6: CONCURRENCY & EDGE CASES ============
func TestEdgeCases(t *testing.T) {
	t.Run("SC12: Multiple transactions same date/time", func(t *testing.T) {
		baseTime := time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)

		// Create multiple transactions at same time (different milliseconds)
		for i := 1; i <= 3; i++ {
			req := services.CreateTransactionRequest{
				OrganizationID: testOrg1ID,
				ItemID:         testItemID,
				TxnDate:        baseTime.Add(time.Duration(i) * time.Millisecond),
				Amount:         10 * i,
				Type:           "penerimaan",
				ChangedBy:      "user1",
			}
			_, err := testService.CreateTransaction(req)
			assertNoError(t, err)
		}

		// Should have correct cumulative balance
		balance, _ := testService.GetCurrentBalance(testOrg1ID, testItemID)
		// Previous 80 + (10 + 20 + 30) = 140
		assertEqual(t, 140, balance)
	})

	t.Run("SC13: Get transactions with pagination", func(t *testing.T) {
		transactions, total, err := testService.GetTransactions(
			testOrg1ID, testItemID,
			time.Time{}, time.Time{}, // No date filter
			1, 10,
		)

		assertNoError(t, err)
		assert.True(t, total > 0)
		assert.True(t, len(transactions) > 0)
		assert.True(t, len(transactions) <= 10)

		// Verify ordering (newest first)
		for i := 0; i < len(transactions)-1; i++ {
			assert.True(t,
				transactions[i].TxnDate.After(transactions[i+1].TxnDate) ||
					(transactions[i].TxnDate.Equal(transactions[i+1].TxnDate) &&
						transactions[i].CreatedAt.After(transactions[i+1].CreatedAt)),
			)
		}
	})

	t.Run("SC14: Invalid transaction types", func(t *testing.T) {
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Now(),
			Amount:         100,
			Type:           "invalid_type", // Invalid
			ChangedBy:      "user1",
		}

		_, err := testService.CreateTransaction(req)
		assertError(t, err, "invalid transaction type")
	})

	t.Run("SC15: Negative amount for penerimaan", func(t *testing.T) {
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg1ID,
			ItemID:         testItemID,
			TxnDate:        time.Now(),
			Amount:         -100, // Negative for penerimaan
			Type:           "penerimaan",
			ChangedBy:      "user1",
		}

		_, err := testService.CreateTransaction(req)
		// Should fail validation
		assert.Error(t, err)
	})
}

// ============ TEST SCENARIO 7: SUMMARY REPORTS ============
func TestSummaryReports(t *testing.T) {
	t.Run("SC16: Organization summary", func(t *testing.T) {
		summary, err := testService.GetOrganizationSummary(testOrg1ID)
		assertNoError(t, err)
		assert.True(t, len(summary) > 0)

		// Find our test item in summary
		found := false
		for _, item := range summary {
			if itemID, ok := item["item_id"].(uint); ok && itemID == testItemID {
				found = true
				assert.Equal(t, "ITEM001", item["item_code"])
				assert.Equal(t, "Test Item", item["item_name"])
				assert.Equal(t, "pcs", item["unit"])
				break
			}
		}
		assert.True(t, found, "Test item should be in summary")
	})

	t.Run("SC17: Item summary across orgs", func(t *testing.T) {
		summary, err := testService.GetItemSummary(testItemID)
		assertNoError(t, err)
		assert.True(t, len(summary) >= 2) // At least our 2 test orgs

		// Verify both orgs appear
		orgsFound := make(map[uuid.UUID]bool)
		for _, org := range summary {
			if orgID, ok := org["organization_id"].(uuid.UUID); ok {
				orgsFound[orgID] = true
			}
		}

		assert.True(t, orgsFound[testOrg1ID], "Org1 should be in summary")
		assert.True(t, orgsFound[testOrg2ID], "Org2 should be in summary")
	})
}

// ============ TEST SCENARIO 8: DATA INTEGRITY ============
func TestDataIntegrity(t *testing.T) {
	t.Run("SC18: History is created for all operations", func(t *testing.T) {
		// Count histories before
		var beforeCount int64
		testDB.Model(&models.InventoryHistory{}).Count(&beforeCount)

		// Create a transaction
		req := services.CreateTransactionRequest{
			OrganizationID: testOrg2ID,
			ItemID:         testItemID,
			TxnDate:        time.Now(),
			Amount:         50,
			Type:           "penerimaan",
			ChangedBy:      "integrity_test",
		}
		inv, _ := testService.CreateTransaction(req)

		// Update it
		updateReq := services.UpdateTransactionRequest{
			InventoryID: inv.ID,
			TxnDate:     time.Now(),
			Amount:      75,
			ChangedBy:   "integrity_test",
		}
		testService.UpdateTransaction(updateReq)

		// Delete it
		testService.DeleteTransaction(inv.ID, "integrity_test", nil)

		// Count histories after
		var afterCount int64
		testDB.Model(&models.InventoryHistory{}).Count(&afterCount)

		// Should have created at least 3 histories (CREATE, UPDATE_BEFORE, DELETE_BEFORE)
		assert.True(t, afterCount-beforeCount >= 3,
			"Expected at least 3 history entries, got %d", afterCount-beforeCount)
	})

	t.Run("SC19: Balance consistency after complex operations", func(t *testing.T) {
		newOrgID := uuid.New()
		testDB.Create(&models.Organization{
			ID:   newOrgID,
			Name: "Integrity Org",
			Code: "ORG999",
		})

		// Complex sequence
		operations := []struct {
			amount int
			type_  string
			date   time.Time
		}{
			{100, "penerimaan", time.Date(2024, 7, 1, 9, 0, 0, 0, time.UTC)},
			{-20, "pemakaian", time.Date(2024, 7, 2, 10, 0, 0, 0, time.UTC)},
			{50, "penerimaan", time.Date(2024, 7, 3, 11, 0, 0, 0, time.UTC)},
			{-30, "pemakaian", time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC)},
		}

		var lastInventoryID uuid.UUID
		for _, op := range operations {
			req := services.CreateTransactionRequest{
				OrganizationID: newOrgID,
				ItemID:         testItemID,
				TxnDate:        op.date,
				Amount:         op.amount,
				Type:           op.type_,
				ChangedBy:      "integrity_test",
			}
			inv, err := testService.CreateTransaction(req)
			if err != nil {
				t.Fatalf("Failed to create transaction: %v", err)
			}
			lastInventoryID = inv.ID
		}

		// Final balance should be: 100 - 20 + 50 - 30 = 100
		finalBalance, err := testService.GetCurrentBalance(newOrgID, testItemID)
		if err != nil {
			t.Fatalf("Failed to get balance: %v", err)
		}
		if finalBalance != 100 {
			t.Fatalf("Expected final balance 100, got %d", finalBalance)
		}

		// Now update the third transaction
		updateReq := services.UpdateTransactionRequest{
			InventoryID: lastInventoryID,
			TxnDate:     time.Date(2024, 7, 4, 12, 0, 0, 0, time.UTC),
			Amount:      -40, // Changed from -30 to -40
			ChangedBy:   "integrity_test",
		}
		err = testService.UpdateTransaction(updateReq)
		if err != nil {
			t.Fatalf("Failed to update transaction: %v", err)
		}

		// New balance should be: 100 - 20 + 50 - 40 = 90
		newBalance, err := testService.GetCurrentBalance(newOrgID, testItemID)
		if err != nil {
			t.Fatalf("Failed to get new balance: %v", err)
		}
		if newBalance != 90 {
			t.Fatalf("Expected new balance 90, got %d", newBalance)
		}

		// Verify all balances in database are consistent
		var transactions []models.Inventory
		testDB.Where("organization_id = ? AND item_id = ? AND deleted_at IS NULL",
			newOrgID, testItemID).
			Order("txn_date ASC, created_at ASC").
			Find(&transactions)

		runningBalance := 0
		for i, tx := range transactions {
			runningBalance += tx.Amount
			if runningBalance != tx.Balance {
				t.Fatalf("Transaction %d has inconsistent balance. Expected %d, got %d",
					i+1, runningBalance, tx.Balance)
			}
		}
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
