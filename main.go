package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

	// Step 1: Check and setup API binary
	if err := setupAPIBinary(); err != nil {
		log.Fatalf("Failed to setup API binary: %v", err)
	}

	// Start API process
	if err := startAPIProcess(); err != nil {
		log.Fatalf("Failed to start API process: %v", err)
	}

	// Wait for API to start
	time.Sleep(2 * time.Second)

	// Setup request forwarding server
	setupForwardingServer()
}

func setupAPIBinary() error {
	// Check if api file exists
	if _, err := os.Stat(apiFileName); os.IsNotExist(err) {
		log.Println("API binary not found, downloading...")
		if err := downloadFile(apiFileName, apiURL); err != nil {
			return fmt.Errorf("failed to download API binary: %v", err)
		}

		// Make file executable
		if err := os.Chmod(apiFileName, 0755); err != nil {
			return fmt.Errorf("failed to set executable permissions: %v", err)
		}
		log.Println("API binary downloaded and setup successfully")
	} else {
		log.Println("API binary already exists")
	}
	return nil
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func startAPIProcess() error {
	cmd := exec.Command("./" + apiFileName)
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
