package models

import "github.com/tokalink/tgo-booster"

// Product represents the product domain entity
type Product struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Price       string `json:"price"`
	Stock       string `json:"stock"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// InitialProducts provides seeded catalog data for starter demonstration
var InitialProducts = []map[string]interface{}{
	{
		"id":          "101",
		"name":        "Apple MacBook Pro 16 M3 Max",
		"price":       "Rp 42.000.000",
		"stock":       "14",
		"status":      "Active",
		"description": "High performance laptop with M3 Max chip and 64GB Unified Memory",
	},
	{
		"id":          "102",
		"name":        "Keychron Q1 Pro Wireless Mechanical Keyboard",
		"price":       "Rp 2.850.000",
		"stock":       "25",
		"status":      "Active",
		"description": "Custom mechanical keyboard with aluminum frame and Bluetooth 5.1",
	},
	{
		"id":          "103",
		"name":        "LG UltraFine 4K 32-inch Ergonomic Display",
		"price":       "Rp 11.500.000",
		"stock":       "4",
		"status":      "Processing",
		"description": "32-inch 4K UHD IPS display with USB-C 90W power delivery",
	},
	{
		"id":          "104",
		"name":        "Sony WH-1000XM5 Noise Cancelling Headphones",
		"price":       "Rp 4.999.000",
		"stock":       "18",
		"status":      "Active",
		"description": "Industry leading active noise cancellation with 30-hour battery life",
	},
	{
		"id":          "105",
		"name":        "Logitech MX Master 3S Wireless Mouse",
		"price":       "Rp 1.650.000",
		"stock":       "32",
		"status":      "Active",
		"description": "Quiet click ergonomic mouse with MagSpeed electromagnetic scrolling",
	},
}

// ProductRepo is the Model DataProvider for products (MemoryStore or SQLStore)
var ProductRepo booster.DataProvider = booster.NewMemoryStore(InitialProducts...)
