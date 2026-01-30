package handlers

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"inventory-ledger/src/requests"
	"inventory-ledger/src/services"
)

type InventoryHandler struct {
	Service *services.InventoryService
}

// ============ GET ENDPOINTS ============

// GetCurrentBalance - Get current balance
func (h *InventoryHandler) GetCurrentBalance(c *gin.Context) {
	orgID, err := uuid.Parse(c.Query("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	itemID, err := strconv.Atoi(c.Query("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	balance, err := h.Service.GetCurrentBalance(orgID, uint(itemID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organization_id": orgID,
		"item_id":         itemID,
		"current_balance": balance,
		"timestamp":       time.Now().Format(time.RFC3339),
	})
}

// GetBalanceAt - Get historical balance
func (h *InventoryHandler) GetBalanceAt(c *gin.Context) {
	orgID, err := uuid.Parse(c.Query("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	itemID, err := strconv.Atoi(c.Query("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	dateStr := c.Query("date")
	var date time.Time

	date, err = time.Parse(time.RFC3339, dateStr)
	if err != nil {
		date, err = time.Parse("2006-01-02", dateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid date format. Use YYYY-MM-DD or RFC3339"})
			return
		}
	}

	balance, err := h.Service.GetBalanceAt(orgID, uint(itemID), date)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organization_id": orgID,
		"item_id":         itemID,
		"balance_at":      balance,
		"as_of_date":      date.Format(time.RFC3339),
	})
}

// GetTransactions - Get transaction history
func (h *InventoryHandler) GetTransactions(c *gin.Context) {
	orgID, err := uuid.Parse(c.Query("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	itemID, err := strconv.Atoi(c.Query("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	var fromDate, toDate time.Time
	if fromStr := c.Query("from_date"); fromStr != "" {
		fromDate, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr := c.Query("to_date"); toStr != "" {
		toDate, _ = time.Parse("2006-01-02", toStr)
		toDate = time.Date(toDate.Year(), toDate.Month(), toDate.Day(), 23, 59, 59, 0, toDate.Location())
	}

	transactions, total, err := h.Service.GetTransactions(
		orgID, uint(itemID), fromDate, toDate, page, limit,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	totalPages := (int(total) + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"data": transactions,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
		},
	})
}

// GetOrganizationSummary - Get org summary
func (h *InventoryHandler) GetOrganizationSummary(c *gin.Context) {
	orgID, err := uuid.Parse(c.Query("organization_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid organization_id"})
		return
	}

	summary, err := h.Service.GetOrganizationSummary(orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"organization_id": orgID,
		"summary":         summary,
		"generated_at":    time.Now().Format(time.RFC3339),
	})
}

// GetItemSummary - Get item summary across all organizations
func (h *InventoryHandler) GetItemSummary(c *gin.Context) {
	itemID, err := strconv.Atoi(c.Query("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item_id"})
		return
	}

	summary, err := h.Service.GetItemSummary(uint(itemID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"item_id":      itemID,
		"summary":      summary,
		"generated_at": time.Now().Format(time.RFC3339),
	})
}

// CreateTransaction - Create inventory transaction
func (h *InventoryHandler) CreateTransaction(c *gin.Context) {
	var req requests.CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnDate, err := time.Parse(time.RFC3339, req.TxnDate)
	if err != nil {
		txnDate, err = time.Parse("2006-01-02T15:04:05", req.TxnDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid txn_date format. Use RFC3339 or YYYY-MM-DDTHH:MM:SS"})
			return
		}
	}

	serviceReq := services.CreateTransactionRequest{
		OrganizationID: req.OrganizationID,
		ItemID:         req.ItemID,
		TxnDate:        txnDate,
		Amount:         req.Amount,
		Type:           req.Type,
		ChangedBy:      req.ChangedBy,
		Reason:         req.Reason,
		RefID:          req.RefID,
		TargetID:       req.TargetID,
		Source:         req.Source,
		PageCode:       req.PageCode,
		Notes:          req.Notes,
	}

	inventory, err := h.Service.CreateTransaction(serviceReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Transaction created successfully",
		"data":    inventory,
	})
}

// ============ MUTATION ============
type MutationRequest struct {
	FromOrganizationID uuid.UUID  `json:"from_organization_id" binding:"required"`
	ToOrganizationID   uuid.UUID  `json:"to_organization_id" binding:"required"`
	ItemID             uint       `json:"item_id" binding:"required"`
	Quantity           int        `json:"quantity" binding:"required,min=1"`
	TxnDate            string     `json:"txn_date" binding:"required"`
	ChangedBy          string     `json:"changed_by" binding:"required"`
	Reason             *string    `json:"reason,omitempty"`
	RefID              *uuid.UUID `json:"ref_id,omitempty"`
	Notes              *string    `json:"notes,omitempty"`
}

// CreateMutation - Create stock mutation
func (h *InventoryHandler) CreateMutation(c *gin.Context) {
	var req MutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	txnDate, err := time.Parse(time.RFC3339, req.TxnDate)
	if err != nil {
		txnDate, err = time.Parse("2006-01-02T15:04:05", req.TxnDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid txn_date format"})
			return
		}
	}

	serviceReq := services.MutationRequest{
		FromOrganizationID: req.FromOrganizationID,
		ToOrganizationID:   req.ToOrganizationID,
		ItemID:             req.ItemID,
		Quantity:           req.Quantity,
		TxnDate:            txnDate,
		ChangedBy:          req.ChangedBy,
		Reason:             req.Reason,
		RefID:              req.RefID,
		Notes:              req.Notes,
	}

	err = h.Service.CreateMutation(serviceReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Mutation completed successfully",
	})
}

// ============ OPNAME ============
type OpnameRequest struct {
	OrganizationID uuid.UUID  `json:"organization_id" binding:"required"`
	ItemID         uint       `json:"item_id" binding:"required"`
	PhysicalQty    int        `json:"physical_qty" binding:"required"`
	TxnDate        string     `json:"txn_date" binding:"required"`
	ChangedBy      string     `json:"changed_by" binding:"required"`
	Reason         *string    `json:"reason,omitempty"`
	RefID          *uuid.UUID `json:"ref_id,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
}

// CreateOpname - Create stock opname
func (h *InventoryHandler) CreateOpname(c *gin.Context) {
	var req OpnameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnDate, err := time.Parse(time.RFC3339, req.TxnDate)
	if err != nil {
		txnDate, err = time.Parse("2006-01-02T15:04:05", req.TxnDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid txn_date format"})
			return
		}
	}

	serviceReq := services.OpnameRequest{
		OrganizationID: req.OrganizationID,
		ItemID:         req.ItemID,
		PhysicalQty:    req.PhysicalQty,
		TxnDate:        txnDate,
		ChangedBy:      req.ChangedBy,
		Reason:         req.Reason,
		RefID:          req.RefID,
		Notes:          req.Notes,
	}

	inventory, err := h.Service.CreateOpname(serviceReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Opname completed successfully",
		"data":    inventory,
	})
}

// ============ UPDATE ============
type UpdateTransactionRequest struct {
	InventoryID uuid.UUID  `json:"inventory_id" binding:"required"`
	TxnDate     string     `json:"txn_date" binding:"required"`
	Amount      int        `json:"amount" binding:"required"`
	ChangedBy   string     `json:"changed_by" binding:"required"`
	Reason      *string    `json:"reason,omitempty"`
	TargetID    *uuid.UUID `json:"target_id,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
}

func (h *InventoryHandler) UpdateTransaction(c *gin.Context) {
	var req UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txnDate, err := time.Parse(time.RFC3339, req.TxnDate)
	if err != nil {
		txnDate, err = time.Parse("2006-01-02T15:04:05", req.TxnDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid txn_date format"})
			return
		}
	}

	serviceReq := services.UpdateTransactionRequest{
		InventoryID: req.InventoryID,
		TxnDate:     txnDate,
		Amount:      req.Amount,
		ChangedBy:   req.ChangedBy,
		Reason:      req.Reason,
		TargetID:    req.TargetID,
		Notes:       req.Notes,
	}

	err = h.Service.UpdateTransaction(serviceReq)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transaction updated successfully",
	})
}

// ============ DELETE ============
type DeleteTransactionRequest struct {
	DeletedBy string  `json:"deleted_by" binding:"required"`
	Reason    *string `json:"reason,omitempty"`
}

func (h *InventoryHandler) DeleteTransaction(c *gin.Context) {
	inventoryID, err := uuid.Parse(c.Query("inventory_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid inventory_id"})
		return
	}

	var req DeleteTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.Service.DeleteTransaction(inventoryID, req.DeletedBy, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Transaction deleted successfully",
	})
}

// RollbackTransaction - Rollback to a specific history point
func (h *InventoryHandler) RollbackTransaction(c *gin.Context) {
	var req requests.RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.Service.RollbackTransaction(req.HistoryID, req.ChangedBy, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "Rollback completed successfully",
		"history_id": req.HistoryID,
	})
}

// GetHistory - Get inventory history for audit trail
func (h *InventoryHandler) GetHistory(c *gin.Context) {
	orgIDStr := c.Query("organization_id")
	itemIDStr := c.Query("item_id")
	action := c.Query("action")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}

	var orgID uuid.UUID
	if orgIDStr != "" {
		var err error
		orgID, err = uuid.Parse(orgIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid organization_id"})
			return
		}
	}

	var itemID uint
	if itemIDStr != "" {
		parsed, err := strconv.ParseUint(itemIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item_id"})
			return
		}
		itemID = uint(parsed)
	}

	history, total, err := h.Service.GetHistory(orgID, itemID, action, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": history,
		"meta": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}
