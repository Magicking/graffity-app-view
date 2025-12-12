package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ERC721Metadata represents the standard ERC721 metadata structure
type ERC721Metadata struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Image       string                 `json:"image"`
	Attributes  []Attribute            `json:"attributes,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	ExternalURL string                 `json:"external_url,omitempty"`
}

type Attribute struct {
	TraitType string      `json:"trait_type"`
	Value     interface{} `json:"value"`
}

// ERC721Service handles ERC721 metadata fetching
type ERC721Service struct {
	client          *ethclient.Client
	contractAddress common.Address
	abi             abi.ABI
}

// tokenURIABI is the ABI for the tokenURI function
const tokenURIABI = `[{"constant":true,"inputs":[{"name":"_tokenId","type":"uint256"}],"name":"tokenURI","outputs":[{"name":"","type":"string"}],"payable":false,"stateMutability":"view","type":"function"}]`

func NewERC721Service(rpcURL, contractAddress string) (*ERC721Service, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(tokenURIABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	return &ERC721Service{
		client:          client,
		contractAddress: common.HexToAddress(contractAddress),
		abi:             parsedABI,
	}, nil
}

// GetChainID retrieves the chain ID from the connected RPC server
func (s *ERC721Service) GetChainID(ctx context.Context) (*big.Int, error) {
	chainID, err := s.client.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID from RPC: %w", err)
	}
	return chainID, nil
}

func (s *ERC721Service) GetTokenMetadata(ctx context.Context, tokenID string) (*ERC721Metadata, error) {
	// Call tokenURI(tokenId) on the contract
	tokenURI, err := s.callTokenURI(ctx, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to get token URI: %w", err)
	}

	// Fetch metadata from the URI
	metadata, err := s.fetchMetadataFromURI(ctx, tokenURI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}

	return metadata, nil
}

func (s *ERC721Service) callTokenURI(ctx context.Context, tokenID string) (string, error) {
	// Convert tokenID string to *big.Int
	tokenIDBig, ok := new(big.Int).SetString(tokenID, 10)
	if !ok {
		return "", fmt.Errorf("invalid token ID: %s", tokenID)
	}

	// Pack the function call
	data, err := s.abi.Pack("tokenURI", tokenIDBig)
	if err != nil {
		return "", fmt.Errorf("failed to pack function call: %w", err)
	}

	// Call the contract
	msg := ethereum.CallMsg{
		To:   &s.contractAddress,
		Data: data,
	}
	result, err := s.client.CallContract(ctx, msg, nil)
	if err != nil {
		return "", fmt.Errorf("contract call failed: %w", err)
	}

	// Unpack the result
	var tokenURI string
	err = s.abi.UnpackIntoInterface(&tokenURI, "tokenURI", result)
	if err != nil {
		return "", fmt.Errorf("failed to unpack result: %w", err)
	}

	return tokenURI, nil
}

func (s *ERC721Service) fetchMetadataFromURI(ctx context.Context, uri string) (*ERC721Metadata, error) {
	// Handle data URIs with base64-encoded JSON
	if strings.HasPrefix(uri, "data:application/json;base64,") {
		return s.parseDataURI(uri)
	}

	// Handle IPFS URIs
	if strings.HasPrefix(uri, "ipfs://") {
		uri = strings.Replace(uri, "ipfs://", "https://ipfs.io/ipfs/", 1)
	} else if strings.HasPrefix(uri, "ipfs/") {
		uri = "https://ipfs.io/" + uri
	}

	// Create HTTP request with timeout
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, "GET", uri, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var metadata ERC721Metadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}

func (s *ERC721Service) parseDataURI(dataURI string) (*ERC721Metadata, error) {
	// Extract the base64-encoded part after the comma
	prefix := "data:application/json;base64,"
	if !strings.HasPrefix(dataURI, prefix) {
		return nil, fmt.Errorf("invalid data URI format")
	}

	base64Data := strings.TrimPrefix(dataURI, prefix)

	// Decode base64
	jsonData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Parse JSON
	var metadata ERC721Metadata
	if err := json.Unmarshal(jsonData, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	return &metadata, nil
}

func (s *ERC721Service) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// FormatMetadataAsText converts ERC721Metadata to a readable text format
// c0-c4 are characters for different brightness levels (c0=lightest, c4=darkest)
func FormatMetadataAsText(metadata *ERC721Metadata, chainID int64, tokenID string, c0, c1, c2, c3, c4 string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ERC721 Token Metadata\n"))
	sb.WriteString(fmt.Sprintf("====================\n\n"))
	sb.WriteString(fmt.Sprintf("Chain ID: %d\n", chainID))
	sb.WriteString(fmt.Sprintf("Token ID: %s\n\n", tokenID))

	if metadata.Name != "" {
		sb.WriteString(fmt.Sprintf("Name: %s\n", metadata.Name))
	}

	if metadata.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", metadata.Description))
	}

	if metadata.Image != "" {
		// Check if image is a base64-encoded BMP
		if strings.HasPrefix(metadata.Image, "data:image/bmp;base64,") {
			bitfield, err := decodeBMPToBitfield(metadata.Image, c0, c1, c2, c3, c4)
			if err == nil {
				sb.WriteString(fmt.Sprintf("Image (BMP Bitfield):\n%s\n", bitfield))
			} else {
				sb.WriteString(fmt.Sprintf("Image: %s\n", metadata.Image))
			}
		} else {
			sb.WriteString(fmt.Sprintf("Image: %s\n", metadata.Image))
		}
	}

	if metadata.ExternalURL != "" {
		// Check if external URL is a base64-encoded BMP
		if strings.HasPrefix(metadata.ExternalURL, "data:image/bmp;base64,") {
			bitfield, err := decodeBMPToBitfield(metadata.ExternalURL, c0, c1, c2, c3, c4)
			if err == nil {
				sb.WriteString(fmt.Sprintf("External URL (BMP Bitfield):\n%s\n", bitfield))
			} else {
				sb.WriteString(fmt.Sprintf("External URL: %s\n", metadata.ExternalURL))
			}
		} else {
			sb.WriteString(fmt.Sprintf("External URL: %s\n", metadata.ExternalURL))
		}
	}
	/*
		if len(metadata.Attributes) > 0 {
			sb.WriteString(fmt.Sprintf("\nAttributes:\n"))
			for _, attr := range metadata.Attributes {
				sb.WriteString(fmt.Sprintf("  - %s: %v\n", attr.TraitType, attr.Value))
			}
		}
	*/
	if len(metadata.Properties) > 0 {
		sb.WriteString(fmt.Sprintf("\nProperties:\n"))
		for key, value := range metadata.Properties {
			sb.WriteString(fmt.Sprintf("  - %s: %v\n", key, value))
		}
	}

	return sb.String()
}

// BMPHeader represents the BMP file header
type BMPHeader struct {
	Signature  uint16 // BM = 0x4D42
	FileSize   uint32
	Reserved   uint32
	DataOffset uint32
}

// BMPInfoHeader represents the BMP info header
type BMPInfoHeader struct {
	Size            uint32
	Width           int32
	Height          int32
	ColorPlanes     uint16
	BitsPerPixel    uint16
	Compression     uint32
	ImageSize       uint32
	HorizontalRes   uint32
	VerticalRes     uint32
	ColorsInPalette uint32
	ImportantColors uint32
}

// decodeBMPToBitfield decodes a base64-encoded BMP image and returns it as a bitfield representation
// c0-c4 are characters for different brightness levels (c0=lightest, c4=darkest)
// For 1-bit images: c0=bit 0, c1=bit 1
// For multi-depth images: brightness is mapped to c0-c4
func decodeBMPToBitfield(dataURI string, c0, c1, c2, c3, c4 string) (string, error) {
	// Extract the base64-encoded part
	prefix := "data:image/bmp;base64,"
	if !strings.HasPrefix(dataURI, prefix) {
		return "", fmt.Errorf("invalid BMP data URI format")
	}

	base64Data := strings.TrimPrefix(dataURI, prefix)

	// Decode base64
	bmpData, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	if len(bmpData) < 54 {
		return "", fmt.Errorf("BMP data too short")
	}

	// Parse BMP header (14 bytes)
	var header BMPHeader
	header.Signature = binary.LittleEndian.Uint16(bmpData[0:2])
	if header.Signature != 0x4D42 { // "BM"
		return "", fmt.Errorf("invalid BMP signature")
	}
	header.FileSize = binary.LittleEndian.Uint32(bmpData[2:6])
	header.Reserved = binary.LittleEndian.Uint32(bmpData[6:10])
	header.DataOffset = binary.LittleEndian.Uint32(bmpData[10:14])

	// Parse BMP info header (40 bytes for BITMAPINFOHEADER)
	var infoHeader BMPInfoHeader
	infoHeader.Size = binary.LittleEndian.Uint32(bmpData[14:18])
	infoHeader.Width = int32(binary.LittleEndian.Uint32(bmpData[18:22]))
	infoHeader.Height = int32(binary.LittleEndian.Uint32(bmpData[22:26]))
	infoHeader.ColorPlanes = binary.LittleEndian.Uint16(bmpData[26:28])
	infoHeader.BitsPerPixel = binary.LittleEndian.Uint16(bmpData[28:30])
	infoHeader.Compression = binary.LittleEndian.Uint32(bmpData[30:34])
	infoHeader.ImageSize = binary.LittleEndian.Uint32(bmpData[34:38])
	infoHeader.HorizontalRes = binary.LittleEndian.Uint32(bmpData[38:42])
	infoHeader.VerticalRes = binary.LittleEndian.Uint32(bmpData[42:46])
	infoHeader.ColorsInPalette = binary.LittleEndian.Uint32(bmpData[46:50])
	infoHeader.ImportantColors = binary.LittleEndian.Uint32(bmpData[50:54])

	// Validate
	if infoHeader.BitsPerPixel != 1 && infoHeader.BitsPerPixel != 4 && infoHeader.BitsPerPixel != 8 && infoHeader.BitsPerPixel != 24 && infoHeader.BitsPerPixel != 32 {
		return "", fmt.Errorf("unsupported bits per pixel: %d", infoHeader.BitsPerPixel)
	}

	width := int(infoHeader.Width)
	height := int(infoHeader.Height)
	isTopDown := height < 0
	if isTopDown {
		height = -height // Top-down BMP
	}

	// Calculate row size (must be multiple of 4 bytes)
	bytesPerPixel := int(infoHeader.BitsPerPixel) / 8
	if infoHeader.BitsPerPixel == 1 {
		bytesPerPixel = 0 // Special handling for 1-bit
	}
	rowSize := ((width*int(infoHeader.BitsPerPixel) + 31) / 32) * 4

	// Extract pixel data
	pixelDataStart := int(header.DataOffset)
	if pixelDataStart+rowSize*height > len(bmpData) {
		return "", fmt.Errorf("BMP data incomplete")
	}

	// Convert to bitfield representation
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Dimensions: %dx%d, BPP: %d\n", width, height, infoHeader.BitsPerPixel))
	sb.WriteString("  Bitfield:\n")

	// For 1-bit images, display as binary
	if infoHeader.BitsPerPixel == 1 {
		// BMP is stored bottom-up unless height is negative (top-down)
		startY := height - 1
		endY := -1
		stepY := -1
		if isTopDown {
			startY = 0
			endY = height
			stepY = 1
		}
		for y := startY; y != endY; y += stepY {
			rowOffset := pixelDataStart + y*rowSize
			sb.WriteString("    ")
			for x := 0; x < width; x++ {
				byteOffset := rowOffset + x/8
				bitOffset := 7 - (x % 8)
				if byteOffset < len(bmpData) {
					bit := (bmpData[byteOffset] >> bitOffset) & 1
					if bit == 1 {
						sb.WriteString(c1)
					} else {
						sb.WriteString(c0)
					}
				}
			}
			sb.WriteString("\n")
		}
	} else {
		// For other bit depths, convert to grayscale and display
		// BMP is stored bottom-up unless height is negative (top-down)
		startY := height - 1
		endY := -1
		stepY := -1
		if isTopDown {
			startY = 0
			endY = height
			stepY = 1
		}
		for y := startY; y != endY; y += stepY {
			rowOffset := pixelDataStart + y*rowSize
			sb.WriteString("    ")
			for x := 0; x < width; x++ {
				pixelOffset := rowOffset + x*bytesPerPixel
				if pixelOffset+bytesPerPixel <= len(bmpData) {
					var brightness byte
					if infoHeader.BitsPerPixel == 8 {
						// Grayscale
						brightness = bmpData[pixelOffset]
					} else if infoHeader.BitsPerPixel == 24 || infoHeader.BitsPerPixel == 32 {
						// BGR or BGRA
						b := bmpData[pixelOffset]
						g := bmpData[pixelOffset+1]
						r := bmpData[pixelOffset+2]
						// Calculate brightness using standard formula
						brightness = byte((int(r)*299 + int(g)*587 + int(b)*114) / 1000)
					} else {
						brightness = 128 // Default for unsupported formats
					}

					// Map brightness to character (c0=lightest, c4=darkest)
					// Divide brightness range (0-255) into 5 levels
					if brightness >= 204 { // 255 * 0.8
						sb.WriteString(c0)
					} else if brightness >= 153 { // 255 * 0.6
						sb.WriteString(c1)
					} else if brightness >= 102 { // 255 * 0.4
						sb.WriteString(c2)
					} else if brightness >= 51 { // 255 * 0.2
						sb.WriteString(c3)
					} else {
						sb.WriteString(c4)
					}
				}
			}
			sb.WriteString("\n")
		}
	}

	return sb.String(), nil
}
