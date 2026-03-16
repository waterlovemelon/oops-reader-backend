package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type BookParseRequest struct {
	Input string `json:"input"`
}

func main() {
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/v1/utils/parse-book-info", parseBookInfoHandler)
	http.HandleFunc("/v1/utils/book-cover", getBookCoverHandler)

	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	log.Printf("Starting server on port %s...\n", port)
	log.Println("Available endpoints:")
	log.Println("  GET  /health")
	log.Println("  POST /v1/utils/parse-book-info")
	log.Println("  GET  /v1/utils/book-cover?book_name=xxx")
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "ok",
		"message": "service is healthy",
	})
}

func parseBookInfoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BookParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	wd, err := getWorkingDir()
	if err != nil {
		http.Error(w, "Failed to get working directory", http.StatusInternalServerError)
		return
	}

	utilsPath := filepath.Join(wd, "utils")
	scriptPath := filepath.Join(utilsPath, "book_parser.py")

	cmd := exec.Command("python3", scriptPath, req.Input)
	cmd.Dir = utilsPath
	cmd.WaitDelay = 20 * time.Second

	output, err := cmd.CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to parse book info",
			"details": err.Error(),
			"output":  string(output),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func getBookCoverHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	bookName := r.URL.Query().Get("book_name")
	if bookName == "" {
		http.Error(w, "book_name query parameter is required", http.StatusBadRequest)
		return
	}

	wd, err := getWorkingDir()
	if err != nil {
		http.Error(w, "Failed to get working directory", http.StatusInternalServerError)
		return
	}

	utilsPath := filepath.Join(wd, "utils")
	bookCoverPath := filepath.Join(utilsPath, "bookCover.py")

	cmd := exec.Command("python3", bookCoverPath, bookName)
	cmd.Dir = utilsPath
	cmd.WaitDelay = 20 * time.Second

	output, err := cmd.CombinedOutput()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to get book cover",
			"details": err.Error(),
			"output":  string(output),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}

func getWorkingDir() (string, error) {
	wd, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}

	if filepath.Base(wd) == "utils" {
		return filepath.Dir(wd), nil
	}

	if filepath.Base(wd) == "cmd" || filepath.Base(wd) == "api" {
		return filepath.Dir(filepath.Dir(wd)), nil
	}

	return wd, nil
}
