package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCreateWallet tests the creation of a wallet.
// It ensures that a wallet is created successfully and returns a valid address.
func TestCreateWallet(t *testing.T) {
	// Test creating a wallet
	walletAddress, err := CreateWallet()
	assert.NoError(t, err, "should not return an error when creating a wallet")
	assert.NotEmpty(t, walletAddress, "wallet address should not be empty")
}

// TestListWallets ensures that the ListWallets function works correctly.
// It starts by checking that no wallets are present, then creates a wallet and verifies that it is listed.
func TestListWallets(t *testing.T) {
	// Ensure there are no wallets initially
	wallets := ListWallets()
	assert.Empty(t, wallets, "wallets list should be empty initially")

	// Create a wallet
	walletAddress, err := CreateWallet()
	assert.NoError(t, err, "should not return an error when creating a wallet")

	// List wallets again and check that the created wallet is present
	wallets = ListWallets()
	assert.Contains(t, wallets, walletAddress, "wallets list should contain the created wallet")
}

// TestSignData tests the SignData function to verify that data can be signed using the wallet.
// It ensures that signing data does not produce errors and generates a non-empty signature.
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

// TestSignDataWalletNotFound ensures that when attempting to sign data with a non-existent wallet,
// an error is returned and no signature is generated.
func TestSignDataWalletNotFound(t *testing.T) {
	// Attempt to sign data with a non-existent wallet
	nonExistentWallet := "0xNonExistentWalletAddress"
	dataToSign := "Test data"
	signature, err := SignData(nonExistentWallet, dataToSign)
	assert.Error(t, err, "should return an error when wallet is not found")
	assert.Empty(t, signature, "signature should be empty when wallet is not found")
}
