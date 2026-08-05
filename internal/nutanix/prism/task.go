// Package prism implements hand-written adapters for the locked Prism API.
package prism

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"

	ntnxtask "github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

// Locked source: Nutanix Developer Portal Prism v4.3 GA getTaskById,
// https://developers.nutanix.com/api/v1/namespaces/prism/versions/v4.3/yaml,
// OpenAPI SHA-256 efb04f7aff22e65b3abaf099d3a4cbd113b27d18375f504ea384ed432bb13976.
const (
	getTaskOperation                 = "prism.get_task_by_id"
	getTaskPath                      = "/api/prism/v4.3/config/tasks/{extId}"
	getTaskSelect                    = "extId,status,progressPercentage,errorMessages,warnings,completionDetails,lastUpdatedTime"
	getTaskBodyLimit           int64 = 4 << 20
	maximumTaskErrors                = 100
	maximumTaskWarnings              = 50
	maximumCompletionItems           = 50
	maximumProjectionBytes           = 128
	maximumCompletionListItems       = 100
	maximumCompletionMapItems        = 20
)

var (
	// ErrInvalidTaskReader identifies an absent task transport dependency.
	ErrInvalidTaskReader = errors.New("prism task reader is invalid")
	// ErrInvalidTaskRequest identifies an invalid task read input.
	ErrInvalidTaskRequest = errors.New("prism task request is invalid")
	// ErrInvalidTaskResponse identifies a malformed or inconsistent task response.
	ErrInvalidTaskResponse = errors.New("prism task response is invalid")
)

// Executor is the transport port required by the Prism task reader.
type Executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Reader reads the locked Prism v4.3 getTaskById projection.
type Reader struct {
	executor Executor
}

// Format prevents the transport dependency from entering diagnostics.
func (Reader) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("prism.Reader(redacted)"))
}

// NewReader returns a task reader without making a network request.
func NewReader(executor Executor) (*Reader, error) {
	if nilInterface(executor) {
		return nil, ErrInvalidTaskReader
	}
	return &Reader{executor: executor}, nil
}

// Read returns one bounded task snapshot through the locked Prism v4.3 operation.
func (r *Reader) Read(ctx context.Context, extID string) (ntnxtask.Snapshot, error) {
	if ctx == nil || !validTaskID(extID) {
		return ntnxtask.Snapshot{}, ErrInvalidTaskRequest
	}
	if r == nil || nilInterface(r.executor) {
		return ntnxtask.Snapshot{}, ErrInvalidTaskReader
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        getTaskOperation,
		Method:           http.MethodGet,
		PathTemplate:     getTaskPath,
		PathParameters:   map[string]string{"extId": extID},
		Query:            url.Values{"$select": {getTaskSelect}},
		ExpectedStatuses: []int{http.StatusOK},
		SuccessBodyLimit: getTaskBodyLimit,
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return ntnxtask.Snapshot{}, ErrInvalidTaskRequest
	}
	response, err := r.executor.Execute(ctx, request)
	if err != nil {
		return ntnxtask.Snapshot{}, err
	}
	return decodeTask(response.Body(), extID)
}

type getTaskResponse struct {
	Data *taskDTO `json:"data"`
}

type taskDTO struct {
	ExtID              string                `json:"extId"`
	Status             string                `json:"status"`
	ProgressPercentage *int32                `json:"progressPercentage"`
	ErrorMessages      []appMessageDTO       `json:"errorMessages"`
	Warnings           []appMessageDTO       `json:"warnings"`
	CompletionDetails  []completionDetailDTO `json:"completionDetails"`
	LastUpdatedTime    *time.Time            `json:"lastUpdatedTime"`
}

type appMessageDTO struct {
	Code       string `json:"code"`
	ErrorGroup string `json:"errorGroup"`
	Severity   string `json:"severity"`
}

type completionDetailDTO struct {
	Name  string                `json:"name"`
	Value completionDetailValue `json:"value"`
}

type completionDetailValue struct{}

func (*completionDetailValue) UnmarshalJSON(data []byte) error {
	if !validCompletionDetailValue(data) {
		return ErrInvalidTaskResponse
	}
	return nil
}

func decodeTask(body []byte, expectedExtID string) (ntnxtask.Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var envelope getTaskResponse
	if err := decoder.Decode(&envelope); err != nil || envelope.Data == nil {
		return ntnxtask.Snapshot{}, ErrInvalidTaskResponse
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return ntnxtask.Snapshot{}, ErrInvalidTaskResponse
	}
	data := envelope.Data
	if data.ExtID != expectedExtID || data.Status == "" ||
		len(data.ErrorMessages) > maximumTaskErrors ||
		len(data.Warnings) > maximumTaskWarnings ||
		len(data.CompletionDetails) > maximumCompletionItems {
		return ntnxtask.Snapshot{}, ErrInvalidTaskResponse
	}
	progress := int32(0)
	if data.ProgressPercentage != nil {
		progress = *data.ProgressPercentage
		if progress < 0 || progress > 100 {
			return ntnxtask.Snapshot{}, ErrInvalidTaskResponse
		}
	}
	var updated time.Time
	if data.LastUpdatedTime != nil {
		updated = *data.LastUpdatedTime
	}
	return ntnxtask.Snapshot{
		ExtID:              data.ExtID,
		Status:             mapTaskStatus(data.Status),
		ProgressPercentage: progress,
		Errors:             projectMessages(data.ErrorMessages),
		Warnings:           projectMessages(data.Warnings),
		LastUpdatedTime:    updated,
	}, nil
}

func mapTaskStatus(status string) ntnxtask.Status {
	switch status {
	case "QUEUED":
		return ntnxtask.StatusQueued
	case "RUNNING":
		return ntnxtask.StatusRunning
	case "CANCELING":
		return ntnxtask.StatusCanceling
	case "SUSPENDED":
		return ntnxtask.StatusSuspended
	case "SUCCEEDED":
		return ntnxtask.StatusSucceeded
	case "FAILED":
		return ntnxtask.StatusFailed
	case "CANCELED":
		return ntnxtask.StatusCanceled
	case "$REDACTED":
		return ntnxtask.StatusRedacted
	case "$UNKNOWN":
		return ntnxtask.StatusUnknown
	default:
		return ntnxtask.StatusUnknown
	}
}

func validCompletionDetailValue(data []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return false
	}
	switch typed := value.(type) {
	case string, bool:
		return true
	case json.Number:
		_, err := typed.Int64()
		return err == nil
	case map[string]any:
		return stringMap(typed)
	case []any:
		return validCompletionArray(typed)
	default:
		return false
	}
}

func stringMap(value map[string]any) bool {
	for _, item := range value {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func validCompletionArray(value []any) bool {
	if len(value) == 0 {
		return true
	}
	switch value[0].(type) {
	case string:
		if len(value) > maximumCompletionListItems {
			return false
		}
		for _, item := range value {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	case json.Number:
		if len(value) > maximumCompletionListItems {
			return false
		}
		for _, item := range value {
			number, ok := item.(json.Number)
			if !ok {
				return false
			}
			if _, err := number.Int64(); err != nil {
				return false
			}
		}
		return true
	case map[string]any:
		if len(value) > maximumCompletionMapItems {
			return false
		}
		for _, item := range value {
			wrapper, ok := item.(map[string]any)
			if !ok || !validMapWrapper(wrapper) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validMapWrapper(wrapper map[string]any) bool {
	if len(wrapper) == 0 {
		return true
	}
	if len(wrapper) != 1 {
		return false
	}
	wrapped, ok := wrapper["map"]
	if !ok {
		return false
	}
	mapped, ok := wrapped.(map[string]any)
	return ok && stringMap(mapped)
}

func projectMessages(messages []appMessageDTO) []ntnxtask.Projection {
	projected := make([]ntnxtask.Projection, 0, len(messages))
	for _, message := range messages {
		projection := ntnxtask.Projection{
			Code:       safeProjectionToken(message.Code),
			ErrorGroup: safeProjectionToken(message.ErrorGroup),
			Severity:   safeSeverity(message.Severity),
		}
		if projection.Code != "" || projection.ErrorGroup != "" || projection.Severity != "" {
			projected = append(projected, projection)
		}
	}
	return projected
}

func safeProjectionToken(value string) string {
	if value == "" || len(value) > maximumProjectionBytes || !utf8.ValidString(value) {
		return ""
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return ""
	}
	return value
}

func safeSeverity(value string) string {
	switch value {
	case "INFO", "WARNING", "ERROR":
		return value
	default:
		return ""
	}
}

func validTaskID(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return false
		}
		if value[index] == '%' && index+2 < len(value) && isHex(value[index+1]) && isHex(value[index+2]) {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func isHex(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}

func nilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
