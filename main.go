package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var serverId string

func main() {
	serverId = generateUUID()
	logrus.WithFields(logrus.Fields{
		"serverId": serverId,
	}).Info("Starting server")
	http.HandleFunc("/writeFile", writeFile)
	http.HandleFunc("/readFile", readFile)
	http.HandleFunc("/listFiles", listFiles)
	http.HandleFunc("/deleteFile", deleteFile)
	http.HandleFunc("/generateFiles", generateFiles)

	http.ListenAndServe(":8081", nil)
}

func generateUUID() string {
	id, _ := uuid.NewRandom()
	return id.String()
}
func writeJSON(w http.ResponseWriter, msg string, requestId string, data interface{}) {
	res := map[string]interface{}{
		"message":   msg,
		"serverId":  serverId,
		"requestId": requestId,
		"data":      data,
	}
	responseData, err := json.Marshal(res)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create JSON response: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(responseData))
}

func writeFile(w http.ResponseWriter, r *http.Request) {
	requestId := generateUUID()
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.FormValue("filePath")
	fileContent := r.FormValue("fileContent")
	logrus.WithFields(logrus.Fields{
		"filePath":    filePath,
		"fileContent": fileContent,
		"requestId":   requestId,
		"serverId":    serverId,
	}).Info("Writing file")

	if filePath == "" {
		http.Error(w, "filePath is required", http.StatusBadRequest)
		return
	}

	// Ensure parent directory exists. If filePath is just a filename in the
	// current working directory, Dir will be "." and we don't need to create it.
	dir := filepath.Dir(filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			http.Error(w, fmt.Sprintf("Unable to create directories: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}

	err := os.WriteFile(filePath, []byte(fileContent), 0644)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to write to file: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	writeJSON(w, "File written successfully", requestId, nil)
}

func readFile(w http.ResponseWriter, r *http.Request) {
	requestId := generateUUID()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.FormValue("filePath")
	logrus.WithFields(logrus.Fields{
		"filePath":  filePath,
		"requestId": requestId,
		"serverId":  serverId,
	}).Info("Reading file")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Unable to read file: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	writeJSON(w, "File read successfully", requestId, map[string]interface{}{
		"fileContent": string(data),
	})
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	requestId := generateUUID()
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dirPath := r.FormValue("dirPath")
	logrus.WithFields(logrus.Fields{
		"dirPath":   dirPath,
		"requestId": requestId,
		"serverId":  serverId,
	}).Info("Listing files")

	files, err := os.ReadDir(dirPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unable to read directory: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	var fileInfoList []map[string]interface{}
	for _, file := range files {
		filePath := path.Join(dirPath, file.Name())
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to get info for file %s: %s", filePath, err.Error()), http.StatusInternalServerError)
			return
		}
		fileInfoList = append(fileInfoList, map[string]interface{}{
			"fileName": file.Name(),
			"size":     fileInfo.Size(), // Size in bytes
		})
	}

	writeJSON(w, "Files listed successfully", requestId, fileInfoList)
}

// New function to handle file deletion
func deleteFile(w http.ResponseWriter, r *http.Request) {
	requestId := generateUUID()
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filePath := r.URL.Query().Get("filePath")
	logrus.WithFields(logrus.Fields{
		"filePath":  filePath,
		"requestId": requestId,
		"serverId":  serverId,
	}).Info("Deleting file")

	err := os.Remove(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Error(w, fmt.Sprintf("File not found: %s", err), http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Unable to delete file: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	writeJSON(w, "File deleted successfully", requestId, nil)
}

func generateFiles(w http.ResponseWriter, r *http.Request) {
	requestId := generateUUID()
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dirPath := r.FormValue("dirPath")
	sizeInMBStr := r.FormValue("sizeInMB")
	sizeInMB, err := strconv.Atoi(sizeInMBStr)
	if err != nil {
		http.Error(w, "Invalid size value", http.StatusBadRequest)
		return
	}

	filesToGenerate := sizeInMB / 10
	remainingSize := sizeInMB % 10

	prefix := strings.ReplaceAll(generateUUID(), "-", "")

	for i := 0; i < filesToGenerate; i++ {
		filePath := path.Join(dirPath, fmt.Sprintf("%s_file_%d.txt", prefix, i+1))
		content := generateContentSize(10) // 10 MB
		err = os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to write to file: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}

	if remainingSize > 0 {
		filePath := path.Join(dirPath, fmt.Sprintf("%s_file_last.txt", prefix))
		content := generateContentSize(remainingSize)
		err = os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unable to write to file: %s", err.Error()), http.StatusInternalServerError)
			return
		}
	}

	writeJSON(w, "Files generated successfully", requestId, nil)
}

func generateContentSize(sizeInMB int) string {
	const chunk = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" // 36 bytes
	chunkSize := len(chunk)
	totalSize := sizeInMB * 1024 * 1024
	repeatCount := totalSize / chunkSize
	return strings.Repeat(chunk, repeatCount)
}
