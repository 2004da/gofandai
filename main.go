package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	apiURL      = "https://github.com/c21xdx/free/releases/download/250221/apiv6"
	apiFileName = "api"
	apiPort     = "8085"
	serverPort  = "8080"
)

func main() {
	// Set up logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Get current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	// Construct full path for API binary
	apiPath := filepath.Join(currentDir, apiFileName)

	// Step 1: Check and setup API binary
	if err := setupAPIBinary(apiPath); err != nil {
		log.Fatalf("Failed to setup API binary: %v", err)
	}

	// Verify file exists before starting
	if _, err := os.Stat(apiPath); err != nil {
		log.Fatalf("API binary not found at %s after setup: %v", apiPath, err)
	}

	// Start API process
	if err := startAPIProcess(apiPath); err != nil {
		log.Fatalf("Failed to start API process: %v", err)
	}

	// Wait for API to start
	log.Println("Waiting for API to start...")
	time.Sleep(2 * time.Second)

	// Setup request forwarding server
	setupForwardingServer()
}

func setupAPIBinary(apiPath string) error {
	// Check if api file exists
	if _, err := os.Stat(apiPath); os.IsNotExist(err) {
		log.Printf("API binary not found at %s, downloading...", apiPath)
		if err := downloadFile(apiPath, apiURL); err != nil {
			return fmt.Errorf("failed to download API binary: %v", err)
		}

		// Make file executable
		if err := os.Chmod(apiPath, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %v", err)
		}
		log.Println("API binary downloaded and setup successfully")
	} else {
		log.Printf("API binary already exists at %s", apiPath)
	}

	// Verify file is executable
	info, err := os.Stat(apiPath)
	if err != nil {
		return fmt.Errorf("failed to stat API binary: %v", err)
	}

	if info.Mode()&0111 == 0 {
		// File exists but is not executable, try to make it executable
		if err := os.Chmod(apiPath, 0755); err != nil {
			return fmt.Errorf("failed to make file executable: %v", err)
		}
	}

	return nil
}

func downloadFile(filepath string, url string) error {
	log.Printf("Downloading from %s to %s", url, filepath)
	
	// Create temporary file for download
	tmpFile := filepath + ".tmp"
	
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %v", err)
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to save downloaded file: %v", err)
	}

	// Close the file before renaming
	out.Close()

	// Rename temporary file to target file
	if err := os.Rename(tmpFile, filepath); err != nil {
		os.Remove(tmpFile) // Clean up temp file
		return fmt.Errorf("failed to rename temporary file: %v", err)
	}

	log.Printf("Download completed successfully to %s", filepath)
	return nil
}

func startAPIProcess(apiPath string) error {
	log.Printf("Attempting to start API from: %s", apiPath)
	
	cmd := exec.Command(apiPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start API process: %v", err)
	}

	// Run in background
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("API process ended with error: %v", err)
		}
	}()

	log.Printf("API process started on port %s", apiPort)
	return nil
}

func setupForwardingServer() {
	http.HandleFunc("/", forwardRequest)
	log.Printf("Starting forwarding server on port %s", serverPort)
	if err := http.ListenAndServe(":"+serverPort, nil); err != nil {
		log.Fatalf("Failed to start forwarding server: %v", err)
	}
}

func forwardRequest(w http.ResponseWriter, r *http.Request) {
	// Create the forwarding URL
	target := fmt.Sprintf("http://localhost:%s%s", apiPort, r.RequestURI)
	
	// Create new request
	proxyReq, err := http.NewRequest(r.Method, target, r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy original headers
	for header, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(header, value)
		}
	}

	// Forward the request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy the response headers
	for header, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(header, value)
		}
	}

	// Copy the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response: %v", err)
	}
}
