package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============ ENUMS & TYPES ============
type InventoryType string

const (
	InventoryTypeStokAwal   InventoryType = "stok_awal"
	InventoryTypePenerimaan InventoryType = "penerimaan"
	InventoryTypePemakaian  InventoryType = "pemakaian"
	InventoryTypeMutation   InventoryType = "mutation"
	InventoryTypeOpname     InventoryType = "opname"
)

type TransactionSource string

const (
	SourcePurchase TransactionSource = "purchase"
	SourceUsage    TransactionSource = "usage"
	SourceAdjust   TransactionSource = "adjustment"
	SourceReturn   TransactionSource = "return"
)

// ============ MAIN INVENTORY MODEL ============
type Inventory struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	// Organization reference
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index:idx_org_item_date"`

	// Item reference
	ItemID uint `gorm:"not null;index:idx_org_item_date"`

	// Transaction data
	TxnDate time.Time     `gorm:"type:timestamp;not null;index:idx_org_item_date"`
	Amount  int           `gorm:"not null"`
	Balance int           `gorm:"not null"`
	Type    InventoryType `gorm:"type:varchar(20);not null"`

	// Reference tracking
	RefID    *uuid.UUID         `gorm:"type:uuid;index"`
	TargetID *uuid.UUID         `gorm:"type:uuid;index"`
	Source   *TransactionSource `gorm:"type:varchar(20)"`

	// Mutation data
	FromOrganizationID *uuid.UUID `gorm:"type:uuid;index"`
	ToOrganizationID   *uuid.UUID `gorm:"type:uuid;index"`

	// Opname data
	PhysicalQty *int `gorm:"type:integer"`
	SystemQty   *int `gorm:"type:integer"`
	Difference  *int `gorm:"type:integer"`

	// Audit trail
	CreatedBy string  `gorm:"type:varchar(100);not null"`
	UpdatedBy *string `gorm:"type:varchar(100)"`
	DeletedBy *string `gorm:"type:varchar(100)"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Metadata
	PageCode string  `gorm:"type:varchar(50)"`
	Notes    *string `gorm:"type:text"`
}

func (Inventory) TableName() string {
	return "inventories"
}

// ============ HISTORY MODEL ============
type InventoryHistory struct {
	ID uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`

	// SPESIFIK org dan item
	OrganizationID uuid.UUID `gorm:"type:uuid;not null;index:idx_org_item_date"`
	ItemID         uint      `gorm:"not null;index:idx_org_item_date"`

	// Reference ke transaksi yang trigger history
	TriggerInventoryID *uuid.UUID `gorm:"type:uuid;index"`

	// Snapshot data
	DataBefore json.RawMessage `gorm:"type:jsonb"`
	DataAfter  json.RawMessage `gorm:"type:jsonb"`

	// Scope of snapshot
	SnapshotFromDate time.Time `gorm:"not null;index:idx_org_item_date"`

	// Context
	Action    string  `gorm:"type:varchar(50);not null"`
	ChangedBy string  `gorm:"type:varchar(100);not null"`
	Reason    *string `gorm:"type:text"`

	CreatedAt time.Time
}

func (InventoryHistory) TableName() string {
	return "inventory_histories"
}

// SnapshotItem untuk history data
type SnapshotItem struct {
	InventoryID uuid.UUID `json:"inventory_id"`
	TxnDate     time.Time `json:"txn_date"`
	Amount      int       `json:"amount"`
	Balance     int       `json:"balance"`
	Type        string    `json:"type"`
	RefID       *string   `json:"ref_id,omitempty"`
}

// ============ SUPPORTING MODELS ============
type Organization struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name      string    `gorm:"type:varchar(100);not null"`
	Code      string    `gorm:"type:varchar(50);unique;not null"`
	CreatedAt time.Time
}

func (Organization) TableName() string {
	return "organizations"
}

type Item struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	Code      string `gorm:"type:varchar(50);unique;not null"`
	Name      string `gorm:"type:varchar(200);not null"`
	Unit      string `gorm:"type:varchar(20);not null"`
	CreatedAt time.Time
}

func (Item) TableName() string {
	return "items"
}
