package routes

import (
	"inventory-ledger/src/handlers"

	"github.com/gin-gonic/gin"
)

func RegisterInventoryRoutes(r *gin.RouterGroup, handler *handlers.InventoryHandler) {
	// GET endpoints
	r.GET("/balance/current", handler.GetCurrentBalance)
	r.GET("/balance/historical", handler.GetBalanceAt)
	r.GET("/transactions", handler.GetTransactions)
	r.GET("/summary/org", handler.GetOrganizationSummary)
	r.GET("/summary/item", handler.GetItemSummary)
	r.GET("/history", handler.GetHistory)

	// POST endpoints
	r.POST("/transaction", handler.CreateTransaction)
	r.POST("/mutation", handler.CreateMutation)
	r.POST("/opname", handler.CreateOpname)

	// PUT endpoint
	r.PUT("/transaction", handler.UpdateTransaction)

	// ROLLBACK endpoint (NEW!)
	r.POST("/rollback", handler.RollbackTransaction)

	// DELETE endpoint
	r.DELETE("/transaction", handler.DeleteTransaction)
}
