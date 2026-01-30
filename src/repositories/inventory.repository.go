package repositories

import (
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"inventory-ledger/src/models"
)

type InventoryRepository struct {
	DB *gorm.DB
}

// GetCurrentBalance - Get current balance for org+item
func (r *InventoryRepository) GetCurrentBalance(orgID uuid.UUID, itemID uint) (int, error) {
	var inventory models.Inventory
	err := r.DB.
		Where("organization_id = ? AND item_id = ? AND deleted_at IS NULL", orgID, itemID).
		Order("txn_date DESC, created_at DESC").
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return inventory.Balance, nil
}

// GetBalanceAt - Get balance at specific date
func (r *InventoryRepository) GetBalanceAt(orgID uuid.UUID, itemID uint, at time.Time) (int, error) {
	var inventory models.Inventory
	err := r.DB.
		Where("organization_id = ? AND item_id = ? AND txn_date <= ? AND deleted_at IS NULL",
			orgID, itemID, at).
		Order("txn_date DESC, created_at DESC").
		First(&inventory).Error

	if err == gorm.ErrRecordNotFound {
		return 0, nil
	}

	if err != nil {
		return 0, err
	}

	return inventory.Balance, nil
}

// GetTransactions - Get transactions with pagination
func (r *InventoryRepository) GetTransactions(orgID uuid.UUID, itemID uint,
	fromDate, toDate time.Time, page, limit int) ([]models.Inventory, int64, error) {

	var transactions []models.Inventory
	var total int64

	query := r.DB.Model(&models.Inventory{}).
		Where("organization_id = ? AND item_id = ? AND deleted_at IS NULL", orgID, itemID)

	if !fromDate.IsZero() {
		query = query.Where("txn_date >= ?", fromDate)
	}
	if !toDate.IsZero() {
		query = query.Where("txn_date <= ?", toDate)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * limit
	err := query.
		Order("txn_date DESC, created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&transactions).Error

	if err != nil {
		return nil, 0, err
	}

	return transactions, total, nil
}

// GetOrganizationSummary - Get summary for all items in org (NO VIEW)
func (r *InventoryRepository) GetOrganizationSummary(orgID uuid.UUID) ([]map[string]interface{}, error) {
	var items []models.Item
	if err := r.DB.Order("code").Find(&items).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)

	for _, item := range items {
		var latestInv models.Inventory
		err := r.DB.
			Where("organization_id = ? AND item_id = ? AND deleted_at IS NULL", orgID, item.ID).
			Order("txn_date DESC, created_at DESC").
			First(&latestInv).Error

		currentStock := 0
		lastTransaction := time.Time{}

		if err == nil {
			currentStock = latestInv.Balance
			lastTransaction = latestInv.TxnDate
		}

		summary := map[string]interface{}{
			"item_id":          item.ID,
			"item_code":        item.Code,
			"item_name":        item.Name,
			"unit":             item.Unit,
			"current_stock":    currentStock,
			"last_transaction": lastTransaction,
		}

		result = append(result, summary)
	}

	return result, nil
}

// GetItemSummary - Get summary for specific item across all orgs
func (r *InventoryRepository) GetItemSummary(itemID uint) ([]map[string]interface{}, error) {
	var orgs []models.Organization
	if err := r.DB.Order("code").Find(&orgs).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)

	for _, org := range orgs {
		var latestInv models.Inventory
		err := r.DB.
			Where("organization_id = ? AND item_id = ? AND deleted_at IS NULL", org.ID, itemID).
			Order("txn_date DESC, created_at DESC").
			First(&latestInv).Error

		currentStock := 0
		lastTransaction := time.Time{}

		if err == nil {
			currentStock = latestInv.Balance
			lastTransaction = latestInv.TxnDate
		}

		summary := map[string]interface{}{
			"organization_id":   org.ID,
			"organization_name": org.Name,
			"organization_code": org.Code,
			"current_stock":     currentStock,
			"last_transaction":  lastTransaction,
		}

		result = append(result, summary)
	}

	return result, nil
}

// RecalculateForward - Recalculate balances from specific date
func (r *InventoryRepository) RecalculateForward(tx *gorm.DB, orgID uuid.UUID, itemID uint, fromDate time.Time) error {
	var startBalance int
	err := tx.Model(&models.Inventory{}).
		Select("balance").
		Where("organization_id = ? AND item_id = ? AND txn_date < ? AND deleted_at IS NULL",
			orgID, itemID, fromDate).
		Order("txn_date DESC, created_at DESC").
		Limit(1).
		Scan(&startBalance).Error

	if err == gorm.ErrRecordNotFound {
		startBalance = 0
	} else if err != nil {
		return err
	}

	log.Printf("RECALC DEBUG: Start balance before %v = %d", fromDate, startBalance)

	var transactions []models.Inventory
	err = tx.
		Where("organization_id = ? AND item_id = ? AND txn_date >= ? AND deleted_at IS NULL",
			orgID, itemID, fromDate).
		Order("txn_date ASC, created_at ASC").
		Find(&transactions).Error

	if err != nil {
		return err
	}

	log.Printf("RECALC DEBUG: Found %d transactions to recalculate", len(transactions))

	currentBalance := startBalance
	batchUpdates := make([]models.Inventory, 0)

	for i, inv := range transactions {
		oldBalance := inv.Balance
		needsUpdate := false

		if inv.Type == models.InventoryTypeOpname {
			systemQty := currentBalance
			var physicalQty int
			if inv.PhysicalQty != nil {
				physicalQty = *inv.PhysicalQty
			} else {
				physicalQty = inv.Balance
			}
			difference := physicalQty - systemQty
			transactions[i].Balance = physicalQty
			transactions[i].Amount = difference
			transactions[i].SystemQty = &systemQty
			transactions[i].PhysicalQty = &physicalQty
			transactions[i].Difference = &difference

			currentBalance = physicalQty

			log.Printf("Opname[%d]: System=%d, Physical=%d, Diff=%d, Balance=%d",
				i+1, systemQty, physicalQty, difference, currentBalance)

			needsUpdate = true
		} else {
			currentBalance += inv.Amount
			transactions[i].Balance = currentBalance

			log.Printf("Normal[%d]: %s %d → New balance = %d",
				i+1, inv.Type, inv.Amount, currentBalance)

			if oldBalance != currentBalance {
				needsUpdate = true
			}
		}

		if needsUpdate {
			log.Printf("🔄 Updating transaction %d: balance from %d to %d",
				i+1, oldBalance, currentBalance)
			batchUpdates = append(batchUpdates, transactions[i])
		}
	}

	if len(batchUpdates) > 0 {
		log.Printf("Saving %d updated transactions", len(batchUpdates))
		for _, inv := range batchUpdates {
			if err := tx.Save(&inv).Error; err != nil {
				return err
			}
		}
	}

	log.Printf("RECALC COMPLETE: Final balance = %d", currentBalance)
	return nil
}
