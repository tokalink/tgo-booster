package models

import "github.com/tokalink/tgo-booster"

// Customer represents the customer entity
type Customer struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

// InitialCustomers provides seeded customer data for starter demonstration
var InitialCustomers = []map[string]interface{}{
	{"id": "201", "name": "Budi Cahyono", "email": "budi.c@enterprise.co.id", "status": "Active"},
	{"id": "202", "name": "Siti Wulandari", "email": "siti.wulandari@gmail.com", "status": "Active"},
	{"id": "203", "name": "Ahmad Ridwan", "email": "aridwan@tokalink.io", "status": "Active"},
	{"id": "204", "name": "Dewi Sartika", "email": "dewi.sartika@startup.id", "status": "Active"},
	{"id": "205", "name": "Hendra Pratama", "email": "hendra.p@cloudtech.com", "status": "Active"},
}

// CustomerRepo is the Model DataProvider for customers (MemoryStore or SQLStore)
var CustomerRepo booster.DataProvider = booster.NewMemoryStore(InitialCustomers...)
