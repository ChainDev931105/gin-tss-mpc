package services

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"math/rand/v2"
	"runtime"
	"sort"
	"sync/atomic"

	// "runtime"
	"sync"

	"github.com/bnb-chain/tss-lib/common"
	"github.com/bnb-chain/tss-lib/ecdsa/keygen"
	"github.com/bnb-chain/tss-lib/ecdsa/signing"
	"github.com/bnb-chain/tss-lib/test"
	"github.com/bnb-chain/tss-lib/tss"
	"github.com/stretchr/testify/assert"
	// "github.com/bnb-chain/tss-lib/ecdsa/signing"
)

// A map to hold wallet addresses and their associated key shares
type Wallet struct {
	keys         []keygen.LocalPartySaveData
	pIDs         []*tss.PartyID
	threshold    int
	participants int
}

var walletDataStore sync.Map

func LoadKeygenTestFixturesRandomSet(qty int, wallet *Wallet) ([]keygen.LocalPartySaveData, tss.SortedPartyIDs, error) {
	keys := make([]keygen.LocalPartySaveData, 0, qty)
	plucked := make(map[int]interface{}, qty)
	for i := 0; len(plucked) < qty; i = (i + 1) % wallet.participants {
		_, have := plucked[i]
		if pluck := rand.Float32() < 0.5; !have && pluck {
			plucked[i] = new(struct{})
		}
	}
	for i := range plucked {
		keys = append(keys, wallet.keys[i])
	}
	partyIDs := make(tss.UnSortedPartyIDs, len(keys))
	j := 0
	for i := range plucked {
		key := keys[j]
		pMoniker := fmt.Sprintf("%d", i+1)
		partyIDs[j] = tss.NewPartyID(pMoniker, pMoniker, key.ShareID)
		j++
	}
	sortedPIDs := tss.SortPartyIDs(partyIDs)
	sort.Slice(keys, func(i, j int) bool { return keys[i].ShareID.Cmp(keys[j].ShareID) == -1 })
	return keys, sortedPIDs, nil
}

// CreateWallet generates a new wallet using TSS
func CreateWallet() (string, error) {
	testThreshold := 2
	testParticipants := 4
	fixtures, pIDs, err := keygen.LoadKeygenTestFixtures(testParticipants)
	if err != nil {
		common.Logger.Info("No test fixtures were found, so the safe primes will be generated from scratch. This may take a while...")
		pIDs = tss.GenerateTestPartyIDs(testParticipants)
	}
	p2pCtx := tss.NewPeerContext(pIDs)
	parties := make([]*keygen.LocalParty, 0, len(pIDs))
	startGR := runtime.NumGoroutine()

	errCh := make(chan *tss.Error, len(pIDs))
	outCh := make(chan tss.Message, len(pIDs))
	endCh := make(chan keygen.LocalPartySaveData, len(pIDs))

	updater := test.SharedPartyUpdater

	// init the parties
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

	var ended int32
keygen:
	for {
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			break keygen
		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil { // broadcast!
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else { // point-to-point!
				if dest[0].Index == msg.GetFrom().Index {
					fmt.Errorf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
					return "", errors.New("party tried to send a message to itself")
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}
		case save := <-endCh:
			// SAVE a test fixture file for this P (if it doesn't already exist)
			// .. here comes a workaround to recover this party's index (it was removed from save data)
			index, _ := save.OriginalIndex()
			fmt.Printf("Save for %d\n", index)
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(pIDs)) {
				fmt.Printf("Start goroutines: %d, End goroutines: %d\n", startGR, runtime.NumGoroutine())
				break keygen
			}
		}
	}

	walletAddress := "0xGeneratedWalletAddress" // TODO: Replace with a unique address generation logic
	// Store the wallet data
	walletDataStore.Store(walletAddress, Wallet{
		keys:         fixtures,
		pIDs:         pIDs,
		threshold:    testThreshold,
		participants: testParticipants,
	})

	return walletAddress, nil
}

// ListWallets returns all wallet addresses
func ListWallets() []string {
	var wallets []string
	walletDataStore.Range(func(key, value interface{}) bool {
		wallets = append(wallets, key.(string))
		return true
	})
	return wallets
}

// SignData signs data using the wallet
func SignData(walletAddress, data string) (string, error) {
	hashedData := sha256.Sum256([]byte(data))
	messageToSign := new(big.Int).SetBytes(hashedData[:])

	_walletData, exist := walletDataStore.Load(walletAddress)
	if !exist {
		return "", errors.New("wallet not found")
	}

	walletData, ok := _walletData.(Wallet)
	if !ok {
		return "", errors.New("invalid wallet data")
	}

	cntPID := walletData.threshold + 1

	keys, signPIDs, err := LoadKeygenTestFixturesRandomSet(cntPID, &walletData)
	if err != nil {
		common.Logger.Error("should load keygen fixtures")
	}

	p2pCtx := tss.NewPeerContext(signPIDs)
	parties := make([]*signing.LocalParty, 0, cntPID)

	errCh := make(chan *tss.Error, cntPID)
	outCh := make(chan tss.Message, cntPID)
	endCh := make(chan common.SignatureData, cntPID)

	updater := test.SharedPartyUpdater

	// init the parties
	for i := 0; i < cntPID; i++ {
		params := tss.NewParameters(tss.S256(), p2pCtx, signPIDs[i], len(signPIDs), walletData.threshold)

		P := signing.NewLocalParty(messageToSign, params, keys[i], outCh, endCh).(*signing.LocalParty)
		parties = append(parties, P)
		go func(P *signing.LocalParty) {
			if err := P.Start(); err != nil {
				errCh <- err
			}
		}(P)
	}

	var ended int32
signing:
	for {
		fmt.Printf("ACTIVE GOROUTINES: %d\n", runtime.NumGoroutine())
		select {
		case err := <-errCh:
			common.Logger.Errorf("Error: %s", err)
			assert.FailNow(nil, err.Error())
			break signing

		case msg := <-outCh:
			dest := msg.GetTo()
			if dest == nil {
				for _, P := range parties {
					if P.PartyID().Index == msg.GetFrom().Index {
						continue
					}
					go updater(P, msg, errCh)
				}
			} else {
				if dest[0].Index == msg.GetFrom().Index {
					common.Logger.Fatalf("party %d tried to send a message to itself (%d)", dest[0].Index, msg.GetFrom().Index)
				}
				go updater(parties[dest[0].Index], msg, errCh)
			}

		case <-endCh:
			atomic.AddInt32(&ended, 1)
			if atomic.LoadInt32(&ended) == int32(len(signPIDs)) {
				common.Logger.Debug("Done. Received signature data from %d participants", ended)

				// R := parties[0].Temp.BigR
				// r := parties[0].Temp.Rx
				// fmt.Printf("sign result: R(%s, %s), r=%s\n", R.X().String(), R.Y().String(), r.String())

				// modN := common.ModInt(tss.S256().Params().N)

				// BEGIN check s correctness
				// sumS := big.NewInt(0)
				// for _, p := range parties {
				// 	sumS = modN.Add(sumS, p.Temp.Si)
				// }
				// fmt.Printf("S: %s\n", sumS.String())
				// END check s correctness

				// BEGIN ECDSA verify
				// pkX, pkY := keys[0].ECDSAPub.X(), keys[0].ECDSAPub.Y()
				// pk := ecdsa.PublicKey{
				// 	Curve: tss.EC(),
				// 	X:     pkX,
				// 	Y:     pkY,
				// }
				// ok := ecdsa.Verify(&pk, messageToSign.Bytes(), R.X(), sumS)
				// assert.True(nil, ok, "ecdsa verify must pass")
				fmt.Print("ECDSA signing test done.")
				// END ECDSA verify
				break signing
			}
		}
	}

	// // Retrieve the wallet's local keygen state
	// walletData, ok := walletDataStore.Load(wallet)
	// if !ok {
	// 	return "", errors.New("wallet data not found")
	// }
	// localState := walletData.(Wallet).KeygenLocalState

	// // Compute the hash of the data to be signed
	// hashedData := sha256.Sum256([]byte(data))

	// // Set up signing parties
	// partyCount := len(localState.Parties)
	// threshold := len(localState.Parties) - 1
	// partyIDs := localState.Parties
	// signData := signing.NewInput(threshold, partyIDs, hashedData[:])

	// // Start signing protocol
	// signature, err := signing.NewLocalParty(signData, nil, &localState)
	// if err != nil {
	// 	return "", err
	// }

	// return signature.String(), nil

	return "", nil
}
