package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Server struct {
	erc721Service *ERC721Service
	config        *Config
}

func NewServer(config *Config) (*Server, error) {
	erc721Service, err := NewERC721Service(config.RPCURL, config.ContractAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create ERC721 service: %w", err)
	}

	return &Server{
		erc721Service: erc721Service,
		config:        config,
	}, nil
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

	// Fetch metadata
	ctx := r.Context()
	metadata, err := s.erc721Service.GetTokenMetadata(ctx, tokenID)
	if err != nil {
		log.Printf("Error fetching metadata for token %s: %v", tokenID, err)
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
	log.Printf("Contract address: %s", s.config.ContractAddress)
	log.Printf("RPC URL: %s", s.config.RPCURL)
	log.Printf("Example: http://localhost:%s/1/1", s.config.AddressPort)

	return http.ListenAndServe(addr, mux)
}

func (s *Server) Close() {
	if s.erc721Service != nil {
		s.erc721Service.Close()
	}
}
