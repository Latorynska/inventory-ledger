package services

import (
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory-ledger/src/models"
	"inventory-ledger/src/repositories"
)

// ============ REQUEST STRUCTS ============
type CreateTransactionRequest struct {
	OrganizationID uuid.UUID
	ItemID         uint
	TxnDate        time.Time
	Amount         int
	Type           string
	ChangedBy      string
	Reason         *string
	RefID          *uuid.UUID
	TargetID       *uuid.UUID
	Source         *string
	PageCode       *string
	Notes          *string
}

type MutationRequest struct {
	FromOrganizationID uuid.UUID
	ToOrganizationID   uuid.UUID
	ItemID             uint
	Quantity           int
	TxnDate            time.Time
	ChangedBy          string
	Reason             *string
	RefID              *uuid.UUID
	Notes              *string
}

type OpnameRequest struct {
	OrganizationID uuid.UUID
	ItemID         uint
	PhysicalQty    int
	TxnDate        time.Time
	ChangedBy      string
	Reason         *string
	RefID          *uuid.UUID
	Notes          *string
}

type UpdateTransactionRequest struct {
	InventoryID uuid.UUID
	TxnDate     time.Time
	Amount      int
	ChangedBy   string
	Reason      *string
	TargetID    *uuid.UUID
	Notes       *string
}

// ============ INVENTORY SERVICE ============
type InventoryService struct {
	DB   *gorm.DB
	Repo *repositories.InventoryRepository
}

// ============ PUBLIC METHODS ============

// GetCurrentBalance - Get current balance
func (s *InventoryService) GetCurrentBalance(orgID uuid.UUID, itemID uint) (int, error) {
	return s.Repo.GetCurrentBalance(orgID, itemID)
}

// GetBalanceAt - Get historical balance
func (s *InventoryService) GetBalanceAt(orgID uuid.UUID, itemID uint, at time.Time) (int, error) {
	return s.Repo.GetBalanceAt(orgID, itemID, at)
}

// GetTransactions - Get transaction history
func (s *InventoryService) GetTransactions(orgID uuid.UUID, itemID uint,
	fromDate, toDate time.Time, page, limit int) ([]models.Inventory, int64, error) {
	return s.Repo.GetTransactions(orgID, itemID, fromDate, toDate, page, limit)
}

// GetOrganizationSummary - Get org summary (handled in repo)
func (s *InventoryService) GetOrganizationSummary(orgID uuid.UUID) ([]map[string]interface{}, error) {
	return s.Repo.GetOrganizationSummary(orgID)
}

// GetItemSummary - Get item summary across all orgs
func (s *InventoryService) GetItemSummary(itemID uint) ([]map[string]interface{}, error) {
	return s.Repo.GetItemSummary(itemID)
}

// CreateTransaction - Create inventory transaction
func (s *InventoryService) CreateTransaction(req CreateTransactionRequest) (*models.Inventory, error) {
	var inventory *models.Inventory

	if req.Amount == 0 {
		return nil, errors.New("amount cannot be zero")
	}
	if req.Type == "pemakaian" && req.Amount > 0 {
		return nil, errors.New("pemakaian amount must be negative")
	}
	if req.Type == "penerimaan" && req.Amount < 0 {
		return nil, errors.New("penerimaan amount must be positive")
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if !isValidTransactionType(req.Type) {
			return errors.New("invalid transaction type")
		}
		if req.Type == "stok_awal" {
			exists, err := s.checkFirstStockExists(tx, req.OrganizationID, req.ItemID)
			if err != nil {
				return err
			}
			if exists {
				return errors.New("stok awal already exists for this item")
			}
		}
		prevBalance, err := s.Repo.GetBalanceAt(req.OrganizationID, req.ItemID, req.TxnDate)
		if err != nil {
			return err
		}
		newBalance := prevBalance + req.Amount
		inventoryType := models.InventoryType(req.Type)
		var source *models.TransactionSource
		if req.Source != nil {
			s := models.TransactionSource(*req.Source)
			source = &s
		}

		pageCode := ""
		if req.PageCode != nil {
			pageCode = *req.PageCode
		}
		inventory = &models.Inventory{
			OrganizationID: req.OrganizationID,
			ItemID:         req.ItemID,
			TxnDate:        req.TxnDate,
			Amount:         req.Amount,
			Balance:        newBalance,
			Type:           inventoryType,
			RefID:          req.RefID,
			TargetID:       req.TargetID,
			Source:         source,
			PageCode:       pageCode,
			Notes:          req.Notes,
			CreatedBy:      req.ChangedBy,
			CreatedAt:      time.Now(),
		}

		if err := tx.Create(inventory).Error; err != nil {
			return err
		}
		if err := s.createHistory(tx, inventory, "CREATE", req.ChangedBy, req.Reason); err != nil {
			return err
		}
		return s.Repo.RecalculateForward(tx, req.OrganizationID, req.ItemID, req.TxnDate)
	})

	return inventory, err
}

// CreateMutation - Create stock mutation
func (s *InventoryService) CreateMutation(req MutationRequest) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		sourceBalance, err := s.Repo.GetBalanceAt(req.FromOrganizationID, req.ItemID, req.TxnDate)
		if err != nil {
			return err
		}

		if sourceBalance < req.Quantity {
			return errors.New("insufficient stock in source organization")
		}
		refID := uuid.New()
		sourcePrevBalance, err := s.Repo.GetBalanceAt(req.FromOrganizationID, req.ItemID, req.TxnDate)
		if err != nil {
			return err
		}
		sourceInv := &models.Inventory{
			OrganizationID:     req.FromOrganizationID,
			ItemID:             req.ItemID,
			TxnDate:            req.TxnDate,
			Amount:             -req.Quantity,
			Balance:            sourcePrevBalance - req.Quantity,
			Type:               models.InventoryTypeMutation,
			RefID:              &refID,
			FromOrganizationID: &req.FromOrganizationID,
			ToOrganizationID:   &req.ToOrganizationID,
			Notes:              req.Notes,
			CreatedBy:          req.ChangedBy,
			CreatedAt:          time.Now(),
		}
		destPrevBalance, err := s.Repo.GetBalanceAt(req.ToOrganizationID, req.ItemID, req.TxnDate)
		if err != nil {
			return err
		}
		destInv := &models.Inventory{
			OrganizationID:     req.ToOrganizationID,
			ItemID:             req.ItemID,
			TxnDate:            req.TxnDate,
			Amount:             req.Quantity,
			Balance:            destPrevBalance + req.Quantity,
			Type:               models.InventoryTypeMutation,
			RefID:              &refID,
			FromOrganizationID: &req.FromOrganizationID,
			ToOrganizationID:   &req.ToOrganizationID,
			Notes:              req.Notes,
			CreatedBy:          req.ChangedBy,
			CreatedAt:          time.Now(),
		}
		if err := tx.Create(sourceInv).Error; err != nil {
			return err
		}
		if err := tx.Create(destInv).Error; err != nil {
			return err
		}
		if err := s.createHistory(tx, sourceInv, "MUTATION_OUT", req.ChangedBy, req.Reason); err != nil {
			return err
		}
		if err := s.createHistory(tx, destInv, "MUTATION_IN", req.ChangedBy, req.Reason); err != nil {
			return err
		}
		if err := s.Repo.RecalculateForward(tx, req.FromOrganizationID, req.ItemID, req.TxnDate); err != nil {
			return err
		}
		return s.Repo.RecalculateForward(tx, req.ToOrganizationID, req.ItemID, req.TxnDate)
	})
}

// CreateOpname - Create stock opname
func (s *InventoryService) CreateOpname(req OpnameRequest) (*models.Inventory, error) {
	var inventory *models.Inventory

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		systemBalance, err := s.Repo.GetBalanceAt(req.OrganizationID, req.ItemID, req.TxnDate)
		if err != nil {
			return err
		}

		difference := req.PhysicalQty - systemBalance

		log.Printf("OPNAME DEBUG: System=%d, Physical=%d, Difference=%d",
			systemBalance, req.PhysicalQty, difference)

		inventory = &models.Inventory{
			OrganizationID: req.OrganizationID,
			ItemID:         req.ItemID,
			TxnDate:        req.TxnDate,
			Amount:         difference,
			Balance:        req.PhysicalQty,
			Type:           models.InventoryTypeOpname,
			RefID:          req.RefID,
			PhysicalQty:    &req.PhysicalQty,
			SystemQty:      &systemBalance,
			Difference:     &difference,
			Notes:          req.Notes,
			CreatedBy:      req.ChangedBy,
			CreatedAt:      time.Now(),
		}

		log.Printf("OPNAME INVENTORY: Amount=%d, Balance=%d",
			inventory.Amount, inventory.Balance)

		if err := tx.Create(inventory).Error; err != nil {
			return err
		}

		if err := s.createHistory(tx, inventory, "OPNAME", req.ChangedBy, req.Reason); err != nil {
			return err
		}

		log.Println("Recalculating forward after opname...")
		return s.Repo.RecalculateForward(tx, req.OrganizationID, req.ItemID, req.TxnDate)
	})

	return inventory, err
}

// UpdateTransaction - Update existing transactio
func (s *InventoryService) UpdateTransaction(req UpdateTransactionRequest) error {
	log.Printf("Starting UpdateTransaction: inventory_id=%v", req.InventoryID)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.Inventory
		if err := tx.First(&existing, req.InventoryID).Error; err != nil {
			return err
		}

		log.Printf("Existing: type=%s, amount=%d, balance=%d, date=%v",
			existing.Type, existing.Amount, existing.Balance, existing.TxnDate)
		if err := s.createHistory(tx, &existing, "UPDATE_BEFORE", req.ChangedBy, req.Reason); err != nil {
			return err
		}

		if existing.Type == models.InventoryTypeOpname {
			return s.handleOpnameUpdate(tx, existing, req)
		}
		existing.DeletedBy = &req.ChangedBy
		existing.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		if err := tx.Save(&existing).Error; err != nil {
			return err
		}
		prevBalance, err := s.getBalanceBeforeDate(tx, existing.OrganizationID,
			existing.ItemID, req.TxnDate, existing.ID)
		if err != nil {
			return err
		}

		log.Printf("Previous balance before %v = %d", req.TxnDate, prevBalance)
		newInventory := models.Inventory{
			OrganizationID: existing.OrganizationID,
			ItemID:         existing.ItemID,
			TxnDate:        req.TxnDate,
			Amount:         req.Amount,
			Balance:        prevBalance + req.Amount,
			Type:           existing.Type,
			RefID:          existing.RefID,
			TargetID:       req.TargetID,
			Source:         existing.Source,
			PageCode:       existing.PageCode,
			Notes:          req.Notes,
			CreatedBy:      req.ChangedBy,
			CreatedAt:      time.Now(),
		}
		if existing.Type == models.InventoryTypeMutation {
			newInventory.FromOrganizationID = existing.FromOrganizationID
			newInventory.ToOrganizationID = existing.ToOrganizationID
		}
		if existing.Type == models.InventoryTypeOpname {
			newInventory.PhysicalQty = existing.PhysicalQty
			newInventory.SystemQty = existing.SystemQty
			newInventory.Difference = existing.Difference
			if existing.PhysicalQty != nil {
				newInventory.Balance = *existing.PhysicalQty
			}
		}

		if err := tx.Create(&newInventory).Error; err != nil {
			return err
		}

		log.Printf("Created new transaction: amount=%d, balance=%d",
			newInventory.Amount, newInventory.Balance)
		if err := s.createHistory(tx, &newInventory, "UPDATE_AFTER", req.ChangedBy, req.Reason); err != nil {
			return err
		}

		earliestDate := existing.TxnDate
		if req.TxnDate.Before(earliestDate) {
			earliestDate = req.TxnDate
		}

		log.Printf("Recalculating from earliest date: %v", earliestDate)

		return s.Repo.RecalculateForward(tx, existing.OrganizationID,
			existing.ItemID, earliestDate)
	})
}

// Helper untuk handle opname update khusus
func (s *InventoryService) handleOpnameUpdate(tx *gorm.DB, existing models.Inventory, req UpdateTransactionRequest) error {
	log.Printf("Opname update detected! Special handling required.")

	prevBalance, err := s.getBalanceBeforeDate(tx, existing.OrganizationID,
		existing.ItemID, req.TxnDate, existing.ID)
	if err != nil {
		return err
	}

	newDifference := req.Amount
	newSystemQty := prevBalance
	newPhysicalQty := prevBalance + req.Amount

	existing.DeletedBy = &req.ChangedBy
	existing.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
	if err := tx.Save(&existing).Error; err != nil {
		return err
	}

	newOpname := models.Inventory{
		OrganizationID: existing.OrganizationID,
		ItemID:         existing.ItemID,
		TxnDate:        req.TxnDate,
		Amount:         req.Amount,
		Balance:        newPhysicalQty,
		Type:           models.InventoryTypeOpname,
		RefID:          existing.RefID,
		PhysicalQty:    &newPhysicalQty,
		SystemQty:      &newSystemQty,
		Difference:     &newDifference,
		Notes:          req.Notes,
		CreatedBy:      req.ChangedBy,
		CreatedAt:      time.Now(),
	}

	if err := tx.Create(&newOpname).Error; err != nil {
		return err
	}

	log.Printf("📝 Created new opname: system_qty=%d, physical_qty=%d, diff=%d, balance=%d",
		newSystemQty, newPhysicalQty, newDifference, newPhysicalQty)

	return s.Repo.RecalculateForward(tx, existing.OrganizationID,
		existing.ItemID, req.TxnDate)
}

// DeleteTransaction - Soft delete transaction
func (s *InventoryService) DeleteTransaction(inventoryID uuid.UUID, deletedBy string, reason *string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {

		var inventory models.Inventory
		if err := tx.First(&inventory, inventoryID).Error; err != nil {
			return err
		}

		if err := s.createHistory(tx, &inventory, "DELETE_BEFORE", deletedBy, reason); err != nil {
			return err
		}

		inventory.DeletedBy = &deletedBy
		inventory.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
		if err := tx.Save(&inventory).Error; err != nil {
			return err
		}

		return s.Repo.RecalculateForward(tx, inventory.OrganizationID,
			inventory.ItemID, inventory.TxnDate)
	})
}

// ============ PRIVATE HELPER METHODS ============

// createHistory - Create history snapshot for org+item
func (s *InventoryService) createHistory(tx *gorm.DB, inventory *models.Inventory, action, changedBy string, reason *string) error {

	var snapshots []models.Inventory
	err := tx.
		Where("organization_id = ? AND item_id = ? AND txn_date >= ? AND deleted_at IS NULL",
			inventory.OrganizationID, inventory.ItemID, inventory.TxnDate).
		Order("txn_date ASC, created_at ASC").
		Find(&snapshots).Error

	if err != nil {
		return err
	}

	var snapshotItems []models.SnapshotItem
	for _, inv := range snapshots {
		item := models.SnapshotItem{
			InventoryID: inv.ID,
			TxnDate:     inv.TxnDate,
			Amount:      inv.Amount,
			Balance:     inv.Balance,
			Type:        string(inv.Type),
		}
		if inv.RefID != nil {
			refStr := inv.RefID.String()
			item.RefID = &refStr
		}
		snapshotItems = append(snapshotItems, item)
	}

	snapshotJSON, err := json.Marshal(snapshotItems)
	if err != nil {
		return err
	}

	history := models.InventoryHistory{
		OrganizationID:     inventory.OrganizationID,
		ItemID:             inventory.ItemID,
		TriggerInventoryID: &inventory.ID,
		SnapshotFromDate:   inventory.TxnDate,
		Action:             action,
		ChangedBy:          changedBy,
		Reason:             reason,
		CreatedAt:          time.Now(),
	}

	if action == "UPDATE_BEFORE" || action == "DELETE_BEFORE" {
		history.DataBefore = json.RawMessage(snapshotJSON)
	} else {
		history.DataAfter = json.RawMessage(snapshotJSON)
	}

	return tx.Create(&history).Error
}

// checkFirstStockExists - Check if stok awal already exists
func (s *InventoryService) checkFirstStockExists(tx *gorm.DB, orgID uuid.UUID, itemID uint) (bool, error) {
	var count int64
	err := tx.Model(&models.Inventory{}).
		Where("organization_id = ? AND item_id = ? AND type = ? AND deleted_at IS NULL",
			orgID, itemID, models.InventoryTypeStokAwal).
		Count(&count).Error

	return count > 0, err
}

// getBalanceBeforeDate - Get balance before specific date (excluding a record)
func (s *InventoryService) getBalanceBeforeDate(tx *gorm.DB, orgID uuid.UUID, itemID uint, date time.Time, excludeID uuid.UUID) (int, error) {

	var balance int
	err := tx.Model(&models.Inventory{}).
		Select("balance").
		Where("organization_id = ? AND item_id = ? AND txn_date < ? AND id != ? AND deleted_at IS NULL",
			orgID, itemID, date, excludeID).
		Order("txn_date DESC, created_at DESC").
		Limit(1).
		Scan(&balance).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}
	return balance, err
}

// RollbackTransaction
func (s *InventoryService) RollbackTransaction(historyID uuid.UUID, changedBy string, reason *string) error {
	log.Printf("Starting RollbackTransaction: history_id=%v", historyID)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		var history models.InventoryHistory
		if err := tx.First(&history, historyID).Error; err != nil {
			return err
		}

		log.Printf("History found: action=%s, snapshot_from=%v",
			history.Action, history.SnapshotFromDate)

		var snapshotItems []models.SnapshotItem

		var snapshotData json.RawMessage
		switch history.Action {
		case "CREATE", "MUTATION_IN", "MUTATION_OUT", "OPNAME":
			snapshotData = history.DataAfter
		case "UPDATE_BEFORE", "DELETE_BEFORE":
			snapshotData = history.DataBefore
		case "UPDATE_AFTER":
			snapshotData = history.DataBefore
		default:
			return errors.New("unsupported history action for rollback")
		}

		if err := json.Unmarshal(snapshotData, &snapshotItems); err != nil {
			return err
		}

		log.Printf("Snapshot contains %d transactions", len(snapshotItems))

		now := time.Now()
		if err := tx.Model(&models.Inventory{}).
			Where(
				"organization_id = ? AND item_id = ? AND txn_date >= ? AND deleted_at IS NULL",
				history.OrganizationID, history.ItemID, history.SnapshotFromDate,
			).
			Updates(map[string]interface{}{
				"deleted_at": now,
				"deleted_by": changedBy + " (rollback_delete)",
			}).Error; err != nil {
			return err
		}

		log.Printf("🧹 Soft-deleted transactions from %v onward", history.SnapshotFromDate)

		for _, item := range snapshotItems {
			inventoryType := models.InventoryType(item.Type)

			newID := uuid.New()

			inventory := models.Inventory{
				ID:             newID,
				OrganizationID: history.OrganizationID,
				ItemID:         history.ItemID,
				TxnDate:        item.TxnDate,
				Amount:         item.Amount,
				Balance:        item.Balance,
				Type:           inventoryType,
				CreatedBy:      changedBy + " (rollback_restore)",
				CreatedAt:      time.Now(),
			}

			if item.RefID != nil {
				refID, err := uuid.Parse(*item.RefID)
				if err == nil {
					inventory.RefID = &refID
				}
			}

			if inventoryType == models.InventoryTypeMutation {
				var original models.Inventory
				if err := tx.Unscoped().
					Where("id = ?", item.InventoryID).
					First(&original).Error; err == nil {
					inventory.FromOrganizationID = original.FromOrganizationID
					inventory.ToOrganizationID = original.ToOrganizationID
				}
			}

			if inventoryType == models.InventoryTypeOpname {
				var original models.Inventory
				if err := tx.Unscoped().
					Where("id = ?", item.InventoryID).
					First(&original).Error; err == nil {
					inventory.PhysicalQty = original.PhysicalQty
					inventory.SystemQty = original.SystemQty
					inventory.Difference = original.Difference
				}
			}

			if err := tx.Create(&inventory).Error; err != nil {
				log.Printf("Error creating restored transaction: %v", err)
				return err
			}

			log.Printf("Recreated transaction: new_id=%v, date=%v, amount=%d, balance=%d",
				newID, item.TxnDate, item.Amount, item.Balance)
		}

		log.Printf("Recalculating forward balances after rollback...")
		if err := s.Repo.RecalculateForward(tx, history.OrganizationID,
			history.ItemID, history.SnapshotFromDate); err != nil {
			return err
		}

		rollbackHistory := models.InventoryHistory{
			OrganizationID:     history.OrganizationID,
			ItemID:             history.ItemID,
			TriggerInventoryID: history.TriggerInventoryID,
			SnapshotFromDate:   history.SnapshotFromDate,
			Action:             "ROLLBACK",
			ChangedBy:          changedBy,
			Reason:             reason,
			CreatedAt:          time.Now(),
		}

		var currentSnapshots []models.Inventory
		if err := tx.Where(
			"organization_id = ? AND item_id = ? AND txn_date >= ? AND deleted_at IS NULL",
			history.OrganizationID, history.ItemID, history.SnapshotFromDate,
		).Order("txn_date ASC, created_at ASC").Find(&currentSnapshots).Error; err != nil {
			return err
		}

		var currentItems []models.SnapshotItem
		for _, inv := range currentSnapshots {
			item := models.SnapshotItem{
				InventoryID: inv.ID,
				TxnDate:     inv.TxnDate,
				Amount:      inv.Amount,
				Balance:     inv.Balance,
				Type:        string(inv.Type),
			}
			if inv.RefID != nil {
				refStr := inv.RefID.String()
				item.RefID = &refStr
			}
			currentItems = append(currentItems, item)
		}

		currentJSON, err := json.Marshal(currentItems)
		if err != nil {
			return err
		}

		rollbackHistory.DataAfter = json.RawMessage(currentJSON)

		var beforeSnapshots []models.Inventory
		if err := tx.Unscoped().
			Where(
				"organization_id = ? AND item_id = ? AND txn_date >= ? AND deleted_at IS NOT NULL",
				history.OrganizationID, history.ItemID, history.SnapshotFromDate,
			).
			Order("txn_date ASC, created_at ASC").
			Find(&beforeSnapshots).Error; err != nil {
			return err
		}

		var beforeItems []models.SnapshotItem
		for _, inv := range beforeSnapshots {
			item := models.SnapshotItem{
				InventoryID: inv.ID,
				TxnDate:     inv.TxnDate,
				Amount:      inv.Amount,
				Balance:     inv.Balance,
				Type:        string(inv.Type),
			}
			if inv.RefID != nil {
				refStr := inv.RefID.String()
				item.RefID = &refStr
			}
			beforeItems = append(beforeItems, item)
		}

		beforeJSON, err := json.Marshal(beforeItems)
		if err != nil {
			return err
		}

		rollbackHistory.DataBefore = json.RawMessage(beforeJSON)

		if err := tx.Create(&rollbackHistory).Error; err != nil {
			return err
		}

		log.Printf("Rollback completed for history %v", historyID)

		return nil
	})
}

// GetHistory - Get inventory history for audit trail
func (s *InventoryService) GetHistory(orgID uuid.UUID, itemID uint, action string, page, limit int) ([]models.InventoryHistory, int64, error) {
	offset := (page - 1) * limit

	query := s.DB.Model(&models.InventoryHistory{})

	if orgID != uuid.Nil {
		query = query.Where("organization_id = ?", orgID)
	}

	if itemID > 0 {
		query = query.Where("item_id = ?", itemID)
	}

	if action != "" {
		query = query.Where("action = ?", action)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var history []models.InventoryHistory
	err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&history).Error

	return history, total, err
}

// isValidTransactionType - Validate transaction type
func isValidTransactionType(t string) bool {
	switch t {
	case "stok_awal", "penerimaan", "pemakaian":
		return true
	default:
		return false
	}
}
