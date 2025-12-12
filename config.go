package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type RPCConfig struct {
	RPCURL          string
	ContractAddress string
}

type Config struct {
	RPCConfigs  map[int64]*RPCConfig // Map chainID -> RPCConfig
	AddressPort string
}

func LoadConfig() (*Config, error) {
	// Try to load .env file, but don't fail if it doesn't exist
	_ = godotenv.Load()

	rpcConfigs := make(map[int64]*RPCConfig)

	// Support multiple RPC servers via environment variables
	// Format: RPC_URL_<CHAIN_ID>=<RPC_URL>,CONTRACT_ADDRESS_<CHAIN_ID>=<CONTRACT_ADDRESS>
	// Example: RPC_URL_1=https://..., CONTRACT_ADDRESS_1=0x...

	// First, try to find all RPC_URL_<CHAIN_ID> entries
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		// Check for RPC_URL_<CHAIN_ID> pattern
		if strings.HasPrefix(key, "RPC_URL_") {
			chainIDStr := strings.TrimPrefix(key, "RPC_URL_")
			chainID, err := strconv.ParseInt(chainIDStr, 10, 64)
			if err != nil {
				continue // Skip invalid chain IDs
			}

			// Get corresponding contract address
			contractKey := fmt.Sprintf("CONTRACT_ADDRESS_%s", chainIDStr)
			contractAddress := os.Getenv(contractKey)
			if contractAddress == "" {
				return nil, fmt.Errorf("CONTRACT_ADDRESS_%s is required when RPC_URL_%s is set", chainIDStr, chainIDStr)
			}

			rpcConfigs[chainID] = &RPCConfig{
				RPCURL:          value,
				ContractAddress: contractAddress,
			}
		}
	}

	// Fallback to legacy single RPC_URL and CONTRACT_ADDRESS for backward compatibility
	if len(rpcConfigs) == 0 {
		rpcURL := os.Getenv("RPC_URL")
		contractAddress := os.Getenv("CONTRACT_ADDRESS")

		if rpcURL == "" || contractAddress == "" {
			return nil, fmt.Errorf("either RPC_URL_<CHAIN_ID> and CONTRACT_ADDRESS_<CHAIN_ID> pairs, or RPC_URL and CONTRACT_ADDRESS are required")
		}

		// For legacy mode, we'll use chainID 0 as default (will be resolved via RPC)
		rpcConfigs[0] = &RPCConfig{
			RPCURL:          rpcURL,
			ContractAddress: contractAddress,
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = ":8080"
	}

	return &Config{
		RPCConfigs:  rpcConfigs,
		AddressPort: port,
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
