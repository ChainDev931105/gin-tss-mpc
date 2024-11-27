package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateWallet(t *testing.T) {
	// Test creating a wallet
	walletAddress, err := CreateWallet()
	assert.NoError(t, err, "should not return an error when creating a wallet")
	assert.NotEmpty(t, walletAddress, "wallet address should not be empty")
}

func TestListWallets(t *testing.T) {
	// Ensure there are no wallets initially
	wallets := ListWallets()
	assert.Empty(t, wallets, "wallets list should be empty initially")

	// Create a wallet
	walletAddress, err := CreateWallet()
	assert.NoError(t, err, "should not return an error when creating a wallet")

	// List wallets again
	wallets = ListWallets()
	assert.Contains(t, wallets, walletAddress, "wallets list should contain the created wallet")
}

func TestSignData(t *testing.T) {
	// Create a wallet
	walletAddress, err := CreateWallet()
	assert.NoError(t, err, "should not return an error when creating a wallet")

	// Sign some data
	dataToSign := "Test data"
	signature, err := SignData(walletAddress, dataToSign)
	assert.NoError(t, err, "should not return an error when signing data")
	assert.NotEmpty(t, signature, "signature should not be empty")
}

func TestSignDataWalletNotFound(t *testing.T) {
	// Attempt to sign data with a non-existent wallet
	nonExistentWallet := "0xNonExistentWalletAddress"
	dataToSign := "Test data"
	signature, err := SignData(nonExistentWallet, dataToSign)
	assert.Error(t, err, "should return an error when wallet is not found")
	assert.Empty(t, signature, "signature should be empty when wallet is not found")
}
