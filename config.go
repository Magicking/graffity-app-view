package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	RPCURL          string
	ContractAddress string
	AddressPort     string
}

func LoadConfig() (*Config, error) {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC_URL environment variable is required")
	}

	contractAddress := os.Getenv("CONTRACT_ADDRESS")
	if contractAddress == "" {
		return nil, fmt.Errorf("CONTRACT_ADDRESS environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	return &Config{
		RPCURL:          rpcURL,
		ContractAddress: contractAddress,
		AddressPort:     port,
	}, nil
}

func ParseChainID(chainIDStr string) (int64, error) {
	chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid chain ID: %w", err)
	}
	return chainID, nil
}

func ParseTokenID(tokenIDStr string) (string, error) {
	// Token IDs can be very large, so we'll keep them as strings
	// and let the contract handle the conversion
	if tokenIDStr == "" {
		return "", fmt.Errorf("token ID cannot be empty")
	}
	return tokenIDStr, nil
}
