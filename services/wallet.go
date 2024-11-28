package services

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/bnb-chain/tss-lib/common"
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/test"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
)

// Wallet holds the data for a wallet, including key shares and participant IDs.
type Wallet struct {
	keys         []*keygen.LocalPartySaveData // The key shares for the wallet
	pIDs         []*tss.PartyID               // The participant IDs for the wallet
	threshold    int                          // The threshold required for signing
	participants int                          // The total number of participants in the wallet
}

var walletDataStore sync.Map // A concurrent map to store wallet data by address

// CreateWallet generates a new wallet using Threshold Signature Scheme (TSS).
// It initializes key shares, participant IDs, and performs the key generation process.
func CreateWallet() (string, error) {
	// Define threshold and participants for the wallet
	testThreshold := 2
	testParticipants := 4

	// Load keygen test fixtures, or generate new ones if not available
	fixtures, pIDs, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		// If fixtures are not found, log a message and generate test participants
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}

	// Create a peer context with the party IDs
	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*keygen.LocalParty, 0, len(pIDs))

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	// Initialize parties and start the key generation process for each participant
	for i := 0; i < len(pIDs); i++ {
		var P *keygen.LocalParty
		params := tss.NewParameters(tss.S256(), p2pCtx, pIDs[i], len(pIDs), testThreshold)
		if i < len(fixtures) {
			P = keygen.NewLocalParty(params, outCh, endCh, fixtures[i].LocalPreParams).(*keygen.LocalParty)
		} else {
			P = keygen.NewLocalParty(params, outCh, endCh).(*keygen.LocalParty)
		}
		parties = append(parties, P)
		go func(P *keygen.LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	keys := make([]*keygen.LocalPartySaveData, 0, len(pIDs))
	var walletAddress string
	var ended int32

	// Wait for the key generation process to complete
keygen:
	for {
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			break keygen
		case msg := <-outCh:
			// Handle message routing (broadcast or point-to-point)
			dest := msg.GetTo()
			if dest == nil {
				// Broadcast the message to all other parties
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				// Point-to-point messaging
				if dest[0].Index == msg.GetFrom().Index {
					return "", errors.New("party tried to send a message to itself")
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}
		case save := <-endCh:
			// Save party data once key generation is completed
			keys = append(keys, &save)

			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				// Once all parties have completed, derive the public key and wallet address
				pkX, pkY := save.ECDSAPub.X(), save.ECDSAPub.Y()
				pk := ecdsa.PublicKey{
					Curve: tss.EC(),
					X:     pkX,
					Y:     pkY,
				}
				walletAddress, err = getAddressFromPublicKey(&pk)
				if err != nil {
					return "", err
				}
				break keygen
			}
		}
	}

	// Store the wallet data
	walletDataStore.Store(walletAddress, Wallet{
		keys:         keys,
		pIDs:         pIDs,
		threshold:    testThreshold,
		participants: testParticipants,
	})

	// Return the wallet address
	return walletAddress, nil
}

// ListWallets returns a list of all wallet addresses stored in the wallet data store.
func ListWallets() []string {
	var wallets []string
	// Iterate over the wallet data store and collect wallet addresses
	walletDataStore.Range(func(key, value interface{}) bool {
		wallets = append(wallets, key.(string))
		return true
	})
	return wallets
}

// SignData signs the provided data using the wallet identified by the wallet address.
// It uses the TSS signing protocol and requires the appropriate number of participants
// to generate a valid signature.
func SignData(walletAddress, data string) (string, error) {
	// Hash the data to be signed
	hashedData := sha256.Sum256([]byte(data))
	messageToSign := new(big.Int).SetBytes(hashedData[:])

	// Retrieve the wallet data
	_walletData, exist := walletDataStore.Load(walletAddress)
	if !exist {
		return "", errors.New("wallet not found")
	}

	// Assert that the wallet data is valid
	walletData, ok := _walletData.(Wallet)
	if !ok {
		return "", errors.New("invalid wallet data")
	}

	// Load test fixtures and participant IDs for signing
	keys, signPIDs, err := loadKeygenTestFixturesRandomSet(walletData.threshold+1, &walletData)
	if err != nil {
		common.Logger.Error("Failed to load keygen fixtures")
	}

	cntPID := len(signPIDs)
	p2pCtx := tss.NewPeerContext(signPIDs)
	parties := make([]*signing.LocalParty, 0, cntPID)

	errCh := make(chan *tss.Error, cntPID)
	outCh := make(chan tss.Message, cntPID)
	endCh := make(chan common.SignatureData, cntPID)

	updater := test.SharedPartyUpdater

	// Initialize parties for signing
	for i := 0; i < cntPID; i++ {
		params := tss.NewParameters(tss.S256(), p2pCtx, signPIDs[i], cntPID, walletData.threshold)

		// Create new local party for each participant
		P := signing.NewLocalParty(messageToSign, params, keys[i], outCh, endCh).(*signing.LocalParty)
		parties = append(parties, P)
		go func(P *signing.LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var signature string
	var ended int32

	// Wait for signing process to complete
signing:
	for {
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(nil, err.Error())
			break signing
		case msg := <-outCh:
			// Handle message routing (broadcast or point-to-point)
			dest := msg.GetTo()
			if dest == nil {
				// Broadcast the message to all other parties
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				// Point-to-point messaging
				if dest[0].Index == msg.GetFrom().Index {
					common.Logger.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}
		case signatureData := <-endCh:
			// Once the signature data is collected from all parties, return the signature
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(signPIDs)) {
				signature = hex.EncodeToString(signatureData.Signature)
				return signature, nil
			}
		}
	}

	return "", errors.New("signing failed")
}
