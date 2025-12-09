# Graffity App View

A modern Go HTTP service that fetches ERC721 NFT metadata and returns it as a text representation.

## Features

- Fetches ERC721 token metadata from any Ethereum-compatible chain
- Returns metadata in a human-readable text format
- Supports IPFS and HTTP metadata URIs
- Configurable via environment variables
- RESTful API with `/CHAIN_ID/TOKEN_ID` endpoint

## Prerequisites

- Go 1.21 or later
- Access to an Ethereum RPC endpoint (Alchemy, Infura, or local node)

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd graffity-app-view
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file (or set environment variables):
```bash
cp .env.example .env
```

Edit `.env` with your configuration:
```
RPC_URL=https://eth-mainnet.g.alchemy.com/v2/YOUR_API_KEY
CONTRACT_ADDRESS=0x1234567890123456789012345678901234567890
PORT=8080
```

## Configuration

The application uses the following environment variables:

- `RPC_URL` (required): Ethereum RPC endpoint URL
- `CONTRACT_ADDRESS` (required): ERC721 contract address (hex format with 0x prefix)
- `PORT` (optional): HTTP server port (defaults to 8080)

## Usage

1. Start the server:
```bash
go run .
```

Or build and run:
```bash
go build -o graffity-app-view
./graffity-app-view
```

2. Access the API:
```bash
# Get token #1 on chain ID 1 (Ethereum Mainnet)
curl http://localhost:8080/1/1

# Get token #42 on chain ID 1
curl http://localhost:8080/1/42

# Customize characters via query parameters (c0-c4 for multi-depth support)
curl "http://localhost:8080/1/1?c0=%20&c1=#"
curl "http://localhost:8080/1/1?c0=0&c1=1"
curl "http://localhost:8080/1/1?c0=%20&c1=.&c2=:&c3=o&c4=O"

# Health check
curl http://localhost:8080/health
```

## API Endpoints

### `GET /CHAIN_ID/TOKEN_ID`

Returns ERC721 token metadata as plain text.

**Path Parameters:**
- `CHAIN_ID`: The chain ID (e.g., 1 for Ethereum Mainnet, 11155111 for Sepolia)
- `TOKEN_ID`: The token ID to fetch

**Query Parameters:**
- `c0` (optional): Character for lightest/brightest pixels (default: ` ` space)
- `c1` (optional): Character for light pixels (default: `░`)
- `c2` (optional): Character for medium pixels (default: `▒`)
- `c3` (optional): Character for dark pixels (default: `▓`)
- `c4` (optional): Character for darkest pixels (default: `█`)

**Note:** When using special characters in query parameters, they should be URL-encoded. For example:
- Space: `%20` or `+`
- Hash: `%23`
- Unicode characters: Use proper URL encoding

**Character Mapping:**
- **1-bit images**: Uses `c0` for bit 0 and `c1` for bit 1
- **Multi-depth images (8-bit, 24-bit, 32-bit)**: Brightness is mapped to `c0` (lightest) through `c4` (darkest)

**Popular character combinations:**
- `c0=%20&c1=#` - Space and hash (for 1-bit)
- `c0=%20&c1=*` - Space and asterisk (for 1-bit)
- `c0=0&c1=1` - Zero and one (for 1-bit)
- `c0=%20&c1=.&c2=:&c3=o&c4=O` - ASCII art progression (for multi-depth)
- `c0=%20&c1=%E2%96%91&c2=%E2%96%92&c3=%E2%96%93&c4=%E2%96%88` - Unicode shade progression (defaults)

**Example Response:**
```
ERC721 Token Metadata
====================

Chain ID: 1
Token ID: 1

Name: My Awesome NFT
Description: This is a description of my NFT
Image: https://ipfs.io/ipfs/QmHash...
External URL: https://example.com/nft/1

Attributes:
  - Rarity: Legendary
  - Power: 100
  - Color: Blue
```

### `GET /health`

Health check endpoint. Returns `OK` if the server is running.

## Project Structure

```
.
├── main.go          # Application entry point
├── config.go        # Configuration management
├── erc721.go        # ERC721 metadata fetching logic
├── server.go        # HTTP server implementation
├── go.mod           # Go module definition
├── .env.example     # Example environment configuration
└── README.md        # This file
```

## How It Works

1. The service connects to an Ethereum RPC endpoint
2. When a request comes in for `/CHAIN_ID/TOKEN_ID`, it:
   - Calls the `tokenURI(tokenId)` function on the configured ERC721 contract
   - Fetches the metadata JSON from the returned URI (supports HTTP and IPFS)
   - Formats the metadata as readable text
   - Returns it to the client

## Error Handling

The service handles various error cases:
- Invalid chain ID or token ID format
- Contract call failures
- Metadata fetch failures
- Invalid JSON responses

All errors are returned with appropriate HTTP status codes.

## License

MIT

