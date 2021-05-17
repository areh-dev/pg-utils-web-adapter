package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// todo: Check log output (for each error response)
// todo: write tests
// todo: do refactoring
// todo: public on GitHub

// Required external utils
const (
	pSql       = "psql"
	pgDump     = "pg_dump"
	pgRestore  = "pg_restore"
	pgCreateDb = "createdb"
)

// Messages
const (
	messageEnvNotSet          = "Environment variables not set"
	messageMethodNotSupported = "Unsupported HTTP method"
	messageNotSufficientData  = "POST data doesn't have sufficient data"
	messageIoError            = "Unknown IO error"
	messageBackupFileNotFound = "Backup file not found"
	messageFileNameNotSet     = "File name doesn't set by request URL"
)

// Status values
const (
	statusOk      = "ok"
	statusError   = "error"
)

type malformedRequest struct {
	status  int
	message string
}

func (mr *malformedRequest) Error() string {
	return mr.message
}

type actionResponse struct {
	Status  string `json:"status"`
	Action  string `json:"action,omitempty"`
	Message string `json:"message,omitempty"`
	File    string `json:"file,omitempty"`
}

type pgConnection struct {
	Host string
	Port string
	Db   string
	User string
	Pass string
}

var pgEnvSet *pgConnection
var useDirStructure bool

func main() {
	if !checkPgUtils() {
		log.Fatal("[ERR] PostgreSQL utils not found")
		return
	}

	pgEnvSet = loadEnvSettings()
	useDirStructure = strings.ToUpper(getEnvVariableWithDefault("USE_DIR_STRUCTURE", "")) == "TRUE"

	log.Printf("[INFO] PostgreSQL connection settings set in environment variables: %v\n", pgEnvSet != nil)

	http.HandleFunc("/status", statusHandler)
	http.HandleFunc("/backup", backupHandler)
	http.HandleFunc("/restore", restoreHandler)

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
	if pgSettings.Host == "" || pgSettings.Db == "" || pgSettings.User == "" {
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
	writeResponse(w, http.StatusOK, actionResponse{Action: "status", Status: statusOk})
}

func backupHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request [%s] URI: %s, handler: %s", r.Method, r.RequestURI, "backupHandler")
	actionName := "backup"
	pgConnection, badHttpRequest := GetConnectionConfig(w, r, actionName)
	if badHttpRequest {
		return
	}

	var fileName string
	if useDirStructure {
		path := fmt.Sprintf("/backups/%s/%s", pgConnection.Host, pgConnection.Db)
		err := os.MkdirAll(path, 0666)
		if err != nil {
			log.Printf("[ERR] Can't create directory: %s, Error: %s", path, err.Error())
			writeResponse(w, http.StatusInternalServerError,
				actionResponse{Action: actionName, Status: statusError, Message: messageIoError})
		}

		fileName = fmt.Sprintf("%s/%s.dump", path, time.Now().Format("20060102_150405"))
	} else {
		fileName = fmt.Sprintf("/backups/%s_%s_%s.dump", pgConnection.Host, pgConnection.Db, time.Now().Format("20060102_150405"))
	}

	args := []string{
		"-h", pgConnection.Host,
		"-p", pgConnection.Port,
		"-U", pgConnection.User,
		"-Fc",
		"--no-password",
		"-v",
		pgConnection.Db,
		"-f", fileName,
	}

	returnExecutionResult(w, actionName, pgDump, args, pgConnection.Pass, true, fileName)
}

func GetConnectionConfig(w http.ResponseWriter, r *http.Request, actionName string) (*pgConnection, bool) {
	var pgConnection pgConnection

	switch r.Method {
	case http.MethodGet: // Use environment configuration
		if pgEnvSet == nil {
			writeResponse(w, http.StatusNotImplemented,
				actionResponse{Action: actionName, Status: statusError, Message: messageEnvNotSet})
			return nil, true
		}
		pgConnection = *pgEnvSet

	case http.MethodPost: // Read configuration from post request
		err := decodeJsonBody(w, r, &pgConnection)
		if err != nil {
			var mr *malformedRequest
			if errors.As(err, &mr) {
				writeResponse(w, mr.status, actionResponse{Action: actionName, Status: statusError, Message: mr.message})
			} else {
				log.Printf("[ERR] Can't parse malformedRequest: %s", err.Error())
				writeResponse(w, http.StatusInternalServerError,
					actionResponse{Action: actionName, Status: statusError, Message: http.StatusText(http.StatusInternalServerError)})
			}
			return nil, true
		}

		if checkSettings(&pgConnection) == nil {
			writeResponse(w, http.StatusBadRequest, actionResponse{Action: actionName, Status: statusError, Message: messageNotSufficientData})
			return nil, true
		}

	default: // Unsupported HTTP method
		writeResponse(w, http.StatusMethodNotAllowed,
			actionResponse{Action: actionName, Status: statusError, Message: messageMethodNotSupported})
		return nil, true
	}

	return &pgConnection, false
}

func restoreHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("[INFO] Request URI: %s, handler: %s", r.RequestURI, "restoreHandler")
	actionName := "restore"

	file := r.URL.Query().Get("file")
	if file == "" {
		writeResponse(w, http.StatusBadRequest, actionResponse{Action: actionName, Status: statusError, Message: messageFileNameNotSet})
		return
	}

	fileExist, err := fileExist(fmt.Sprintf("/backups/%s", strings.TrimLeft(file, "/.")))
	if err != nil || !fileExist {
		writeResponse(w, http.StatusBadRequest, actionResponse{Action: actionName, Status: statusError, Message: messageBackupFileNotFound})
		return
	}

	pgConnection, badHttpRequest := GetConnectionConfig(w, r, actionName)
	if badHttpRequest {
		return
	}

	var dbExist bool
	dbExist, err = isDbExist(pgConnection)
	if err != nil {
		writeInternalServerErrorResponse(w, actionName, err)
		return
	}

	if dbExist {
		err = restoreDb(pgConnection, file, true)
	} else {
		err = createDb(pgConnection)
		if err != nil {
			writeInternalServerErrorResponse(w, actionName, err)
			return
		}
		err = restoreDb(pgConnection, file, false)
	}

	if err != nil {
		writeInternalServerErrorResponse(w, actionName, err)
		return
	}

	writeResponse(w, http.StatusOK, actionResponse{Action: actionName, Status: statusOk})
}

func restoreDb(pgConnection *pgConnection, dumpFile string, cleanDb bool) error {
	args := []string{
		"-h", pgConnection.Host,
		"-p", pgConnection.Port,
		"-U", pgConnection.User,
		"--no-password",
	}

	if cleanDb {
		args = append(args, "--clean")
	}

	args = append(args, "-d", pgConnection.Db, dumpFile)

	res, out := executeWithOutput(pgRestore, args, pgConnection.Pass, true, true)
	if !res {
		return errors.New(fmt.Sprintf("restoreDb execution error\n%s", out))
	}

	return nil
}

func createDb(pgConnection *pgConnection) error {
	args := []string{
		"-h", pgConnection.Host,
		"-p", pgConnection.Port,
		"-U", pgConnection.User,
		"--no-password",
		"--echo",
		"--template=template0",
		"--encoding=UTF8",
		pgConnection.Db,
	}

	res, out := executeWithOutput(pgCreateDb, args, pgConnection.Pass, true, true)
	if !res {
		return errors.New(fmt.Sprintf("createdb execution error\n%s", out))
	}

	return nil
}

func isDbExist(pgConnection *pgConnection) (bool, error) {
	args := []string{
		"-h", pgConnection.Host,
		"-p", pgConnection.Port,
		"-U", pgConnection.User,
		"--no-password",
		"--tuples-only",
		"--no-align",
		"-c", fmt.Sprintf("\"SELECT 1 FROM pg_database WHERE datname='%s'\"", pgConnection.Db),
	}

	res, out := executeWithOutput(pSql, args, pgConnection.Pass, true, false)
	if !res {
		return false, errors.New(fmt.Sprintf("psql check DB exist execution error\n%s", out))
	}

	return strings.TrimSpace(out) == "1", nil
}

func returnExecutionResult(w http.ResponseWriter, actionName, app string, args []string, pgPassword string, omitSuccessfulOutput bool, fileName string) {
	status := statusError
	httpStatus := http.StatusInternalServerError
	resultFile := ""

	res, out := executeWithOutput(app, args, pgPassword, true, omitSuccessfulOutput)
	if res {
		status = statusOk
		httpStatus = http.StatusOK
		resultFile = fileName
	} else if fileName != "" {
		_ = os.Remove(fileName)
	}

	writeResponse(w, httpStatus, actionResponse{Action: actionName, Status: status, Message: out, File: resultFile})
}

func writeInternalServerErrorResponse(w http.ResponseWriter, actionName string, err error) {
	writeResponse(w, http.StatusInternalServerError,
		actionResponse{Action: actionName, Status: statusError, Message: fmt.Sprintf("%v", err)})
}

func writeResponse(w http.ResponseWriter, responseStatus int, responseData actionResponse) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(responseStatus)

	err := json.NewEncoder(w).Encode(responseData)
	if err != nil {
		log.Printf("[WARN] failed to send response, %v", err)
	}
}

func checkPgUtils() bool {
	args := []string{"--help"}
	return execute(pSql, args, "") &&
		execute(pgDump, args, "") &&
		execute(pgRestore, args, "") &&
		execute(pgCreateDb, args, "")
}

func execute(app string, args []string, pgPassword string) bool {
	res, _ := executeWithOutput(app, args, pgPassword, false, true)
	return res
}

func executeWithOutput(app string, args []string, pgPassword string, printOutput bool, omitSuccessfulOutputMessage bool) (bool, string) {
	cmd := exec.Command(app, args...)

	if pgPassword != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+pgPassword)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		errMessage := fmt.Sprintf("Can't execute app %v, error: %v\nOutput:\n%v", app, err.Error(), string(out))
		fmt.Println("[ERR] " + errMessage)
		return false, errMessage
	}

	if printOutput {
		outputOffset := "\n       --> "
		formattedOutput := outputOffset + strings.ReplaceAll(strings.TrimSpace(string(out)), "\n", outputOffset)
		fmt.Printf("[INFO] External app output:%v\n", formattedOutput)
	}

	if omitSuccessfulOutputMessage {
		return true, ""
	}

	return true, string(out)
}

func decodeJsonBody(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)

	dec := json.NewDecoder(r.Body)
	err := dec.Decode(&dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		case errors.As(err, &syntaxError):
			msg := fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, message: msg}

		case errors.Is(err, io.ErrUnexpectedEOF):
			msg := fmt.Sprintf("Request body contains badly-formed JSON")
			return &malformedRequest{status: http.StatusBadRequest, message: msg}

		case errors.As(err, &unmarshalTypeError):
			msg := fmt.Sprintf("Request body contains an invalid value for the %q field (at position %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			return &malformedRequest{status: http.StatusBadRequest, message: msg}

		case errors.Is(err, io.EOF):
			msg := "Request body must not be empty"
			return &malformedRequest{status: http.StatusBadRequest, message: msg}

		case err.Error() == "http: request body too large":
			msg := "Request body must not be larger than 1MB"
			return &malformedRequest{status: http.StatusRequestEntityTooLarge, message: msg}

		default:
			return err
		}
	}

	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		msg := "Request body must only contain a single JSON object"
		return &malformedRequest{status: http.StatusBadRequest, message: msg}
	}

	return nil
}

func fileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
