package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type Server struct {
	services map[int64]*ERC721Service // Map chainID -> ERC721Service
	config   *Config
	mu       sync.RWMutex
}

func NewServer(config *Config) (*Server, error) {
	services := make(map[int64]*ERC721Service)

	// Create services for each configured chain
	for chainID, rpcConfig := range config.RPCConfigs {
		service, err := NewERC721Service(rpcConfig.RPCURL, rpcConfig.ContractAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to create ERC721 service for chain %d: %w", chainID, err)
		}

		// If chainID is 0 (legacy mode), resolve it from RPC
		if chainID == 0 {
			ctx := context.Background()
			actualChainID, err := service.GetChainID(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get chain ID from RPC: %w", err)
			}
			chainID = actualChainID.Int64()
			log.Printf("Resolved chain ID %d from RPC", chainID)
		}

		services[chainID] = service
		log.Printf("Configured chain %d: RPC=%s, Contract=%s", chainID, rpcConfig.RPCURL, rpcConfig.ContractAddress)
	}

	return &Server{
		services: services,
		config:   config,
	}, nil
}

// getServiceForChainID returns the ERC721Service for the given chain ID
// If not found, it attempts to find a service by querying RPC servers for their chain IDs
func (s *Server) getServiceForChainID(ctx context.Context, chainID int64) (*ERC721Service, error) {
	s.mu.RLock()
	service, exists := s.services[chainID]
	s.mu.RUnlock()

	if exists {
		return service, nil
	}

	// Try to find service by querying RPC servers
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if service, exists := s.services[chainID]; exists {
		return service, nil
	}

	// Query all services to find matching chain ID
	for existingChainID, existingService := range s.services {
		serviceChainID, err := existingService.GetChainID(ctx)
		if err != nil {
			log.Printf("Warning: failed to get chain ID from service for chain %d: %v", existingChainID, err)
			continue
		}

		if serviceChainID.Int64() == chainID {
			// Cache the mapping
			s.services[chainID] = existingService
			log.Printf("Found matching RPC server for chain ID %d", chainID)
			return existingService, nil
		}
	}

	return nil, fmt.Errorf("no RPC server configured for chain ID %d", chainID)
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	// Parse path: /CHAIN_ID/TOKEN_ID
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if len(parts) != 2 {
		http.Error(w, "Invalid path. Expected format: /CHAIN_ID/TOKEN_ID", http.StatusBadRequest)
		return
	}

	chainIDStr := parts[0]
	tokenIDStr := parts[1]

	chainID, err := ParseChainID(chainIDStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid chain ID: %v", err), http.StatusBadRequest)
		return
	}

	tokenID, err := ParseTokenID(tokenIDStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid token ID: %v", err), http.StatusBadRequest)
		return
	}

	// TokenID will be validated and converted in the ERC721Service

	// Parse query parameters for character mapping (c0-c4)
	chars := make([]string, 5)
	defaultChars := []string{" ", "░", "▒", "▓", "█"} // Default: light to dark

	for i := 0; i < 5; i++ {
		paramName := fmt.Sprintf("c%d", i)
		char := r.URL.Query().Get(paramName)

		if char == "" {
			chars[i] = defaultChars[i]
		} else {
			// URL decode the character
			decoded, err := url.QueryUnescape(char)
			if err == nil {
				chars[i] = decoded
			} else {
				chars[i] = defaultChars[i]
			}
		}
	}

	// Get the appropriate service for this chain ID
	ctx := r.Context()
	service, err := s.getServiceForChainID(ctx, chainID)
	if err != nil {
		log.Printf("Error getting service for chain %d: %v", chainID, err)
		http.Error(w, fmt.Sprintf("Failed to get RPC service for chain ID %d: %v", chainID, err), http.StatusBadRequest)
		return
	}

	// Fetch metadata
	metadata, err := service.GetTokenMetadata(ctx, tokenID)
	if err != nil {
		log.Printf("Error fetching metadata for token %s on chain %d: %v", tokenID, chainID, err)
		http.Error(w, fmt.Sprintf("Failed to fetch metadata: %v", err), http.StatusInternalServerError)
		return
	}

	// Format as text
	text := FormatMetadataAsText(metadata, chainID, tokenID, chars[0], chars[1], chars[2], chars[3], chars[4])

	// Set content type and return
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(text))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleToken)
	mux.HandleFunc("/health", s.handleHealth)

	addr := s.config.AddressPort
	log.Printf("Starting server on %s", addr)
	log.Printf("Configured %d chain(s)", len(s.services))
	for chainID := range s.services {
		log.Printf("  Chain %d: configured", chainID)
	}
	log.Printf("Example: http://localhost:%s/1/1", s.config.AddressPort)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for chainID, service := range s.services {
		if service != nil {
			service.Close()
			log.Printf("Closed service for chain %d", chainID)
		}
	}
}
