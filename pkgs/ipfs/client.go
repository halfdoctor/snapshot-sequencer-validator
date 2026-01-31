package ipfs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	files "github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/ipfs/go-cid"
	ipfsApi "github.com/ipfs/kubo/client/rpc"
	log "github.com/sirupsen/logrus"
)

// Client wraps the IPFS kubo client
type Client struct {
	api        *ipfsApi.HttpApi
	httpClient *http.Client
	apiURL     string // Store API URL for direct HTTP calls
	isLocal    bool   // True if connecting to local IPFS node (has /data/ipfs mount)
}

// NewClient creates a new IPFS client
func NewClient(apiURL string) (*Client, error) {
	if apiURL == "" {
		apiURL = "127.0.0.1:5001" // Default IPFS API endpoint
	}

	// Detect if this is a local IPFS node (has /data/ipfs mount) vs external
	// Local: ipfs:5001, localhost:5001, 127.0.0.1:5001
	// External: /dns/hostname/tcp/5001, http://hostname:5001
	isLocal := false

	// Handle different input formats
	if strings.HasPrefix(apiURL, "/ip4/") || strings.HasPrefix(apiURL, "/dns/") {
		// Multiaddr format - external IPFS (unless it's localhost)
		parts := strings.Split(apiURL, "/")
		if len(parts) >= 5 {
			host := parts[2]
			port := parts[4]
			apiURL = fmt.Sprintf("http://%s:%s", host, port)
			// Local if it's localhost IP
			isLocal = (parts[1] == "ip4" && (host == "127.0.0.1" || host == "localhost"))
		}
	} else if !strings.HasPrefix(apiURL, "http://") && !strings.HasPrefix(apiURL, "https://") {
		// Simple host:port format
		host := apiURL
		if idx := strings.Index(apiURL, ":"); idx != -1 {
			host = apiURL[:idx]
		}
		isLocal = (host == "ipfs" || host == "localhost" || host == "127.0.0.1" || host == "")
		apiURL = "http://" + apiURL
	} else {
		// Already has http:// or https://
		if strings.Contains(apiURL, "localhost") || strings.Contains(apiURL, "127.0.0.1") || strings.Contains(apiURL, "ipfs:") {
			isLocal = true
		}
	}

	// Create HTTP client with reasonable timeouts
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:       10,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true,
		},
	}

	// Note: We don't check IPFS_PATH here - it's only relevant for the IPFS service container
	// Aggregator always uses HTTP API, detection of local vs external is based on hostname

	api, err := ipfsApi.NewURLApiWithClient(apiURL, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create IPFS HTTP API client for %s: %w", apiURL, err)
	}

	// CRITICAL: Verify we're actually connecting via HTTP API, not local filesystem
	// Test connection to ensure it's actually connecting to the remote endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get node ID to verify HTTP connection works
	_, err = api.Key().Self(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to IPFS HTTP API at %s: %w (check if IPFS node is accessible, DNS resolution works, and IPFS_PATH env var is not set)", apiURL, err)
	}

	if isLocal {
		log.Infof("✅ Successfully connected to LOCAL IPFS HTTP API at %s (using Unixfs)", apiURL)
	} else {
		log.Infof("✅ Successfully connected to EXTERNAL IPFS HTTP API at %s (using direct HTTP POST)", apiURL)
	}

	return &Client{
		api:        api,
		httpClient: httpClient,
		apiURL:     apiURL,
		isLocal:    isLocal,
	}, nil
}

// StoreFinalizedBatch stores a finalized batch in IPFS and returns the CID
// Strategy:
// - Local IPFS (ipfs:5001): Use Unixfs().Add() - IPFS node handles storage efficiently with /data/ipfs mount
// - External IPFS: Use direct HTTP POST - avoids temp files in aggregator container
func (c *Client) StoreFinalizedBatch(ctx context.Context, batch interface{}) (string, error) {
	// Marshal batch to JSON
	jsonData, err := json.Marshal(batch)
	if err != nil {
		return "", fmt.Errorf("failed to marshal batch: %w", err)
	}

	batchSize := len(jsonData)
	log.Debugf("Storing finalized batch to IPFS (size: %d bytes, ~%.2f KB, local=%v)", batchSize, float64(batchSize)/1024, c.isLocal)

	var cidStr string

	if c.isLocal {
		// Local IPFS: Use Unixfs().Add() - IPFS node has /data/ipfs mount, handles chunking efficiently
		cidStr, err = c.addViaUnixfs(ctx, jsonData)
	} else {
		// External IPFS: Use direct HTTP POST to avoid temp files in aggregator container
		cidStr, err = c.addViaDirectHTTP(ctx, jsonData)
	}

	if err != nil {
		return "", fmt.Errorf("failed to add to IPFS: %w", err)
	}

	log.Debugf("IPFS upload successful: %s", cidStr)
	return cidStr, nil
}

// addViaUnixfs uses Unixfs().Add() for local IPFS nodes
// Local IPFS nodes have /data/ipfs mounted, so they handle storage efficiently
func (c *Client) addViaUnixfs(ctx context.Context, data []byte) (string, error) {
	reader := bytes.NewReader(data)

	// Use Unixfs().Add() - for local IPFS, this is efficient as the node handles storage
	path, err := c.api.Unixfs().Add(ctx, files.NewReaderFile(reader))
	if err != nil {
		return "", fmt.Errorf("unixfs add failed: %w", err)
	}

	// Pin the content
	if err := c.pinCID(ctx, path.String()); err != nil {
		log.WithError(err).Warn("Failed to pin CID, but upload succeeded")
	}

	return path.String(), nil
}

// addViaDirectHTTP makes a direct HTTP POST to /api/v0/add endpoint
// This avoids temp file creation by streaming data directly in multipart/form-data
func (c *Client) addViaDirectHTTP(ctx context.Context, data []byte) (string, error) {
	// Build the API endpoint URL
	apiEndpoint := strings.TrimSuffix(c.apiURL, "/") + "/api/v0/add"

	// Create multipart form with file data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "batch.json")
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := fileWriter.Write(data); err != nil {
		return "", fmt.Errorf("failed to write file data: %w", err)
	}

	// Add query parameters as form fields
	// CID version 1, pin=true
	if err := writer.WriteField("cid-version", "1"); err != nil {
		return "", fmt.Errorf("failed to write cid-version field: %w", err)
	}
	if err := writer.WriteField("pin", "true"); err != nil {
		return "", fmt.Errorf("failed to write pin field: %w", err)
	}
	if err := writer.WriteField("chunker", "size-262144"); err != nil {
		return "", fmt.Errorf("failed to write chunker field: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", apiEndpoint, &requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("IPFS API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response - IPFS returns JSON with "Hash" field
	var result struct {
		Name string `json:"Name"`
		Hash string `json:"Hash"`
		Size string `json:"Size"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode IPFS response: %w", err)
	}

	if result.Hash == "" {
		return "", fmt.Errorf("IPFS API returned empty hash")
	}

	// Return CID with /ipfs/ prefix for consistency
	return "/ipfs/" + result.Hash, nil
}

// pinCID pins a CID to ensure it's not garbage collected
func (c *Client) pinCID(ctx context.Context, cidStr string) error {
	// Remove /ipfs/ prefix if present
	cidStr = strings.TrimPrefix(cidStr, "/ipfs/")

	// Build pin endpoint URL with query parameters
	// Some IPFS implementations expect "ipfs-path" parameter instead of "arg"
	pinEndpoint := strings.TrimSuffix(c.apiURL, "/") + "/api/v0/pin/add"
	
	// Build URL with query parameters
	u, err := url.Parse(pinEndpoint)
	if err != nil {
		return fmt.Errorf("failed to parse pin endpoint URL: %w", err)
	}
	
	q := u.Query()
	// Try both parameter names for compatibility with different IPFS implementations
	q.Set("arg", cidStr)
	q.Set("ipfs-path", cidStr)
	q.Set("recursive", "false")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create pin request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pin request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pin API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RetrieveFinalizedBatch retrieves a finalized batch from IPFS by CID
func (c *Client) RetrieveFinalizedBatch(ctx context.Context, cidStr string) ([]byte, error) {
	// Parse CID
	parsedCID, err := cid.Parse(cidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid CID %s: %w", cidStr, err)
	}

	// Get the content from IPFS using boxo path
	ipfsPath := path.FromCid(parsedCID)
	reader, err := c.api.Unixfs().Get(ctx, ipfsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get from IPFS: %w", err)
	}

	// Read as file
	file := files.ToFile(reader)
	if file == nil {
		return nil, fmt.Errorf("expected file from IPFS")
	}
	defer file.Close()

	// Read all content
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(file); err != nil {
		return nil, fmt.Errorf("failed to read content: %w", err)
	}

	return buf.Bytes(), nil
}

// IsAvailable checks if IPFS node is accessible
func (c *Client) IsAvailable(ctx context.Context) bool {
	// Try to get the node ID as a connectivity check
	_, err := c.api.Key().Self(ctx)
	return err == nil
}
