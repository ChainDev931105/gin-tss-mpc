
# TSS Wallet Server

This is a simple Go application that uses a REST API to create TSS wallets, list them, and sign data.

## Installation

1. Install Go (https://golang.org/).
2. Clone this repository.
3. Run `go mod init tss-wallet-server` and `go mod tidy` to install dependencies.

## Usage

Start the server:
```bash
go run main.go
```

API Endpoints:
- `POST /wallet`: Creates a new wallet and returns the address.
- `GET /wallets`: Returns a list of all created wallets.
- `GET /sign?data=<>&wallet=<>`: Signs data using the wallet and returns the signature.

Examples:
```bash
curl -X POST http://localhost:8080/wallet
curl http://localhost:8080/wallets
curl "http://localhost:8080/sign?data=hello&wallet=0xGeneratedWalletAddress"
```

## Notes

Please modify this part.
``` go
    // wallet.go#L36

	// MODIFY ME
	testThreshold := 2
	testParticipants := 4
```
