package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type BookParseRequest struct {
	Input string `json:"input"`
}

// stripANSICodes 移除 ANSI 控制字符和终端格式字符
func stripANSICodes(input string) string {
	// 移除 ANSI 颜色代码: \x1b[...m 或 \033[...m
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*m|\033\[[0-9;]*m`)
	cleaned := ansiEscapeRegex.ReplaceAllString(input, "")

	// 移除其他控制字符 (0-31 和 127)
	var result bytes.Buffer
	for _, r := range cleaned {
		if r >= 32 && r < 127 || r == '\n' {
			result.WriteRune(r)
		}
	}

	return result.String()
}

// tryParseJSON 尝试解析 JSON 格式
func tryParseJSON(input string) (map[string]interface{}, bool) {
	input = strings.TrimSpace(input)
	var result map[string]interface{}
	err := json.Unmarshal([]byte(input), &result)
	if err != nil {
		return map[string]interface{}{
			"error":   "Failed to parse JSON",
			"message": "Output is not in JSON format",
			"raw":    stripANSICodes(input),
		}, false
	}
	return result, true
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to parse book info",
			"details": err.Error(),
			"output": stripANSICodes(string(output)),
		})
		return
	}

	// 清理输出并返回 JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	jsonResult, isJSON := tryParseJSON(stripANSICodes(string(output)))
	if isJSON {
		json.NewEncoder(w).Encode(jsonResult)
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to parse JSON output",
			"details": "Python script output is not in JSON format",
			"cleaned_output": stripANSICodes(string(output)),
		})
	}
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
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to get book cover",
			"details": err.Error(),
			"output": stripANSICodes(string(output)),
		})
		return
	}

	// 清理输出并返回 JSON
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	jsonResult, isJSON := tryParseJSON(stripANSICodes(string(output)))
	if isJSON {
		json.NewEncoder(w).Encode(jsonResult)
	} else {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "Failed to parse JSON output",
			"details": "Python script output is not in JSON format",
			"cleaned_output": stripANSICodes(string(output)),
		})
	}
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
