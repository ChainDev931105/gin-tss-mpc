package services

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"sort"

	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/tss"
	"golang.org/x/crypto/sha3"
)

// getAddressFromPublicKey derives the Ethereum address from the given ECDSA public key.
// It follows the Ethereum process of serializing the public key, hashing it with Keccak-256,
// and then extracting the last 20 bytes to form the address.
func getAddressFromPublicKey(pk *ecdsa.PublicKey) (string, error) {
	// Serialize the public key in uncompressed form (X, Y coordinates)
	publicKeyBytes := elliptic.Marshal(pk.Curve, pk.X, pk.Y)

	// Remove the first byte (0x04) to work with the 64-byte (X + Y) representation
	publicKeyBytes = publicKeyBytes[1:]

	// Hash the public key using Keccak-256 (Ethereum uses this hash function)
	hash := sha3.NewLegacyKeccak256()
	hash.Write(publicKeyBytes)
	hashed := hash.Sum(nil)

	// The Ethereum address is derived from the last 20 bytes of the Keccak-256 hash
	address := hashed[12:] // Last 20 bytes

	// Return the address as a hex string with the "0x" prefix
	return "0x" + hex.EncodeToString(address), nil
}

// loadKeygenTestFixturesRandomSet loads a random set of keygen test fixtures, selecting participants
// from the wallet based on the given quantity. It ensures that only the required number of key shares
// are selected, and the keyshare data is returned in a sorted order.
func loadKeygenTestFixturesRandomSet(qty int, wallet *Wallet) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs, error) {
	keys := make([]keygen.LocalPartySaveData, 0, qty)
	plucked := make(map[int]interface{}, qty)

	// Randomly select participants
	for i := 0; len(plucked) < qty; i = (i + 1) % wallet.participants {
		_, have := plucked[i]
		if pluck := rand.Float32() < 0.5; !have && pluck {
			plucked[i] = new(struct{})
		}
	}

	// Collect the selected keys
	for i := range plucked {
		key := wallet.keys[i]
		// Ensure the curve for BigXj is set to S256
		for _, kbxj := range key.BigXj {
			kbxj.SetCurve(tss.S256())
		}
		key.ECDSAPub.SetCurve(tss.S256()) // Set the curve for ECDSA public key
		keys = append(keys, *key)
	}

	// Generate Party IDs for the selected participants
	partyIDs := make(tss.UnSortedPartyIDs, len(keys))
	j := 0
	for i := range plucked {
		key := keys[j]
		// Assign a moniker (ID) for the party
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[j] = tss.NewPartyID(pMoniker, pMoniker, key.ShareID)
		j++
	}

	// Sort Party IDs based on the share IDs
	sortedPIDs := tss.SortPartyIDs(partyIDs)

	// Sort the keys based on their share ID
	sort.Slice(keys, func(i, j int) bool { return keys[i].ShareID.Cmp(keys[j].ShareID) == -1 })

	// Return the sorted keys and party IDs
	return keys, sortedPIDs, nil
}
