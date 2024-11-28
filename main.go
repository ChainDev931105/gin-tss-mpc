package main

import (
	"github.com/ChainDev931105/gin-tss-mpc/handlers"
	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize the Gin router
	router := gin.Default()

	// API Endpoints
	router.POST("/wallet", handlers.CreateWallet)
	router.GET("/wallets", handlers.ListWallets)
	router.GET("/sign", handlers.SignData)

	// Start the server on port 8080
	router.Run(":8080")
}
