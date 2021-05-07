package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

type pgConnection struct {
	pgHost string
	pgPort string
	pgDb   string
	pgUser string
	pgPass string
}

var pgEnvSet *pgConnection

func main() {
	pgEnvSet = loadEnvSettings()

	log.Printf("[INFO] PostgreSQL connection settings set in environment variables: %v\n", pgEnvSet != nil)

	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/backup", backupHandler)
	http.HandleFunc("/backup-db", backupFullHandler)
	http.HandleFunc("/restore", restoreHandler)
	http.HandleFunc("/restore-db", restoreFullHandler)

	log.Println("[INFO] Listening port 80")
	err := http.ListenAndServe(":80", nil)
	if err != nil {
		log.Fatal("[ERR] ListenAndServe: ", err)
	}
}

func loadEnvSettings() *pgConnection {
	result := pgConnection{
		getEnvVariableWithDefault("PG_HOST", ""),
		getEnvVariableWithDefault("PG_PORT", "5432"),
		getEnvVariableWithDefault("PG_DB", ""),
		getEnvVariableWithDefault("PG_USER", ""),
		getEnvVariableWithDefault("PG_PASS", "")}

	return checkSettings(&result)
}

func checkSettings(pgSettings *pgConnection) *pgConnection {
	if pgSettings.pgHost == "" || pgSettings.pgDb == "" || pgSettings.pgUser == "" {
		return nil
	}
	return pgSettings
}

func getEnvVariableWithDefault(envVariable, defaultValue string) string {
	value := os.Getenv(envVariable)
	if value == "" {
		return defaultValue
	}
	return value
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "statusHandler")
	writeResponse(w, http.StatusOK, `{"status": "ok"}`)
}

func backupHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "backupHandler")

	if pgEnvSet == nil {
		writeResponse(w, http.StatusNotImplemented, `{"action": "backup", "status": "environment not set"}`)
		return
	}

	// do default backup

	writeResponse(w, http.StatusOK, `{"action": "backup", "status": "skipped"}`)
}

func backupFullHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "backupFullHandler")

	// do backup

	writeResponse(w, http.StatusOK, `{"action": "backupFull", "status": "skipped"}`)
}

func restoreHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "restoreHandler")

	if pgEnvSet == nil {
		writeResponse(w, http.StatusNotImplemented, `{"action": "restore", "status": "environment not set"}`)
		return
	}

	// do default restore

	writeResponse(w, http.StatusOK, `{"action": "restore", "status": "skipped"}`)
}

func restoreFullHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "restoreFullHandler")

	// do restore

	writeResponse(w, http.StatusOK, `{"action": "restoreFull", "status": "skipped"}`)
}

func writeResponse(w http.ResponseWriter, responseStatus int, responseText string) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(responseStatus)

	_, err := fmt.Fprintf(w, responseText)
	if err != nil {
		log.Printf("[WARN] failed to send response, %v", err)
	}
}
