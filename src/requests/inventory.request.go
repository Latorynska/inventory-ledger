package requests

import (
	"time"

	"github.com/google/uuid"
)

// ============ BASE REQUEST ============
type BaseInventoryRequest struct {
	ChangedBy string  `json:"changed_by" binding:"required"`
	Reason    *string `json:"reason,omitempty"`
}

// ============ CREATE/UPDATE ============
type InventoryRequest struct {
	BaseInventoryRequest

	OrganizationID uuid.UUID `json:"organization_id" binding:"required"`
	ItemID         uint      `json:"item_id" binding:"required"`
	TxnDate        time.Time `json:"txn_date" binding:"required"`
	Amount         int       `json:"amount" binding:"required"`
	Type           string    `json:"type" binding:"required,oneof=penerimaan pemakaian stok_awal"`

	// Optional fields
	RefID    *uuid.UUID `json:"ref_id,omitempty"`
	TargetID *uuid.UUID `json:"target_id,omitempty"`
	Source   *string    `json:"source,omitempty"`
	PageCode *string    `json:"page_code,omitempty"`
	Notes    *string    `json:"notes,omitempty"`
}

// ============ MUTATION ============
type MutationRequest struct {
	BaseInventoryRequest

	FromOrganizationID uuid.UUID `json:"from_organization_id" binding:"required"`
	ToOrganizationID   uuid.UUID `json:"to_organization_id" binding:"required"`
	ItemID             uint      `json:"item_id" binding:"required"`
	Quantity           int       `json:"quantity" binding:"required,min=1"`
	TxnDate            time.Time `json:"txn_date" binding:"required"`

	RefID *uuid.UUID `json:"ref_id,omitempty"`
	Notes *string    `json:"notes,omitempty"`
}

// ============ OPNAME ============
type OpnameRequest struct {
	BaseInventoryRequest

	OrganizationID uuid.UUID `json:"organization_id" binding:"required"`
	ItemID         uint      `json:"item_id" binding:"required"`
	PhysicalQty    int       `json:"physical_qty" binding:"required"`
	TxnDate        time.Time `json:"txn_date" binding:"required"`

	RefID *uuid.UUID `json:"ref_id,omitempty"`
	Notes *string    `json:"notes,omitempty"`
}

// ============ UPDATE REQUEST ============
type UpdateInventoryRequest struct {
	BaseInventoryRequest

	InventoryID uuid.UUID `json:"inventory_id" binding:"required"`
	TxnDate     time.Time `json:"txn_date" binding:"required"`
	Amount      int       `json:"amount" binding:"required"`

	// Optional updates
	TargetID *uuid.UUID `json:"target_id,omitempty"`
	Notes    *string    `json:"notes,omitempty"`
}

// ============ ROLLBACK REQUEST ============
type RollbackRequest struct {
	BaseInventoryRequest

	HistoryID uuid.UUID `json:"history_id" binding:"required"`
}

// ============ REQUEST STRUCTS ============
type CreateTransactionRequest struct {
	OrganizationID uuid.UUID  `json:"organization_id" binding:"required"`
	ItemID         uint       `json:"item_id" binding:"required"`
	TxnDate        string     `json:"txn_date" binding:"required"`
	Amount         int        `json:"amount" binding:"required"`
	Type           string     `json:"type" binding:"required,oneof=stok_awal penerimaan pemakaian"`
	ChangedBy      string     `json:"changed_by" binding:"required"`
	Reason         *string    `json:"reason,omitempty"`
	RefID          *uuid.UUID `json:"ref_id,omitempty"`
	TargetID       *uuid.UUID `json:"target_id,omitempty"`
	Source         *string    `json:"source,omitempty"`
	PageCode       *string    `json:"page_code,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
}
