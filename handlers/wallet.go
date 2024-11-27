package handlers

import (
	"net/http"

	"github.com/ChainDev931105/gin-tss-mpc/services"

	"github.com/gin-gonic/gin"
)

// CreateWallet creates a new wallet
func CreateWallet(c *gin.Context) {
	walletAddress, err := services.CreateWallet()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"address": walletAddress})
}

// ListWallets lists all wallets
func ListWallets(c *gin.Context) {
	wallets := services.ListWallets()
	c.JSON(http.StatusOK, gin.H{"wallets": wallets})
}

// SignData signs data using a wallet
func SignData(c *gin.Context) {
	wallet := c.Query("wallet")
	data := c.Query("data")

	if wallet == "" || data == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wallet and data are required"})
		return
	}

	signature, err := services.SignData(wallet, data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"signature": signature})
}
