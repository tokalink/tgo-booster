package models

import "github.com/tokalink/tgo-booster"

// Order represents an order transaction entity
type Order struct {
	ID           string `json:"id"`
	CustomerName string `json:"customer_name"`
	TotalAmount  string `json:"total_amount"`
	Status       string `json:"status"`
}

// InitialOrders provides seeded order transaction records
var InitialOrders = []map[string]interface{}{
	{"id": "ORD-9024", "customer_name": "Budi Cahyono", "total_amount": "Rp 4.850.000", "status": "Completed"},
	{"id": "ORD-9023", "customer_name": "Siti Wulandari", "total_amount": "Rp 12.400.000", "status": "In Transit"},
	{"id": "ORD-9022", "customer_name": "Ahmad Ridwan", "total_amount": "Rp 1.250.000", "status": "Processing"},
	{"id": "ORD-9021", "customer_name": "Dewi Sartika", "total_amount": "Rp 42.000.000", "status": "Completed"},
}

// OrderRepo is the Model DataProvider for orders (MemoryStore or SQLStore)
var OrderRepo booster.DataProvider = booster.NewMemoryStore(InitialOrders...)
