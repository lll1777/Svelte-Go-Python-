package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"sync"
	"time"

	"agriculture-platform/config"
	"agriculture-platform/models"
)

type SafetyCheckStatus string

const (
	SafetyCheckPassed     SafetyCheckStatus = "PASSED"
	SafetyCheckFailed     SafetyCheckStatus = "FAILED"
	SafetyCheckTimeout    SafetyCheckStatus = "TIMEOUT"
	SafetyCheckError      SafetyCheckStatus = "ERROR"
	SafetyCheckCircuitOpen SafetyCheckStatus = "CIRCUIT_OPEN"
)

type CircuitBreakerState int

const (
	CircuitClosed CircuitBreakerState = iota
	CircuitOpen
	CircuitHalfOpen
)

type SafetyCheckAuditLog struct {
	Timestamp       time.Time
	WorkOrderID     string
	ExpertID        string
	Medications     []string
	CheckStatus     SafetyCheckStatus
	IsSafe          bool
	Warnings        string
	ErrorDetails    string
	Attempts        int
	ResponseTime    time.Duration
}

type PrescriptionSafetyResult struct {
	IsSafe           bool
	Warnings         string
	Suggestions      string
	Error            error
	IsFallback       bool
	CheckStatus      SafetyCheckStatus
	CheckTimestamp   time.Time
	ServiceAvailable bool
}

type ImageService struct {
	pythonServiceURL    string
	httpClient          *http.Client
	config              *ServiceConfig
	
	circuitBreakerMutex sync.RWMutex
	circuitState        CircuitBreakerState
	circuitOpenTime     time.Time
	consecutiveFailures int
	
	auditLogMutex       sync.Mutex
	auditLogs           []SafetyCheckAuditLog
}

type ServiceConfig struct {
	DiagnosisTimeout        time.Duration
	PrescriptionTimeout     time.Duration
	MaxRetries              int
	RetryInterval           time.Duration
	FailOpen                bool
	
	CircuitBreakerEnabled   bool
	CircuitFailureThreshold int
	CircuitOpenDuration     time.Duration
	MaxAuditLogs            int
}

var DefaultServiceConfig = &ServiceConfig{
	DiagnosisTimeout:        30 * time.Second,
	PrescriptionTimeout:     15 * time.Second,
	MaxRetries:              3,
	RetryInterval:           2 * time.Second,
	FailOpen:                false,
	
	CircuitBreakerEnabled:   true,
	CircuitFailureThreshold: 5,
	CircuitOpenDuration:     60 * time.Second,
	MaxAuditLogs:            1000,
}

type DiagnosisResponse struct {
	Success        bool    `json:"success"`
	WorkOrderID    string  `json:"work_order_id"`
	DiseaseName    string  `json:"disease_name"`
	DiseaseType    string  `json:"disease_type"`
	Confidence     float64 `json:"confidence"`
	Symptoms       string  `json:"symptoms"`
	Causes         string  `json:"causes"`
	RecommendedActions string `json:"recommended_actions"`
	Severity       string    `json:"severity"`
	SimilarCases   string    `json:"similar_cases"`
	ImageHash      string    `json:"image_hash"`
}

type PrescriptionCheckResponse struct {
	Success     bool   `json:"success"`
	IsSafe      bool   `json:"is_safe"`
	Warnings    string `json:"warnings"`
	Suggestions string `json:"suggestions"`
}

type SimilarCasesResponse struct {
	Success bool   `json:"success"`
	Cases   string `json:"cases"`
}

type ServiceError struct {
	Operation string
	Err       error
	IsTimeout bool
}

func (e *ServiceError) Error() string {
	if e.IsTimeout {
		return fmt.Sprintf("%s: request timeout", e.Operation)
	}
	return fmt.Sprintf("%s: %v", e.Operation, e.Err)
}

func (e *ServiceError) Unwrap() error {
	return e.Err
}

func NewImageService(cfg *config.Config) *ImageService {
	return &ImageService{
		pythonServiceURL: cfg.PythonServiceURL,
		config:           DefaultServiceConfig,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 10 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second,
				MaxIdleConns:          10,
				IdleConnTimeout:       60 * time.Second,
			},
		},
		circuitState: CircuitClosed,
		auditLogs:    make([]SafetyCheckAuditLog, 0, 100),
	}
}

func NewImageServiceWithConfig(cfg *config.Config, serviceCfg *ServiceConfig) *ImageService {
	service := NewImageService(cfg)
	if serviceCfg != nil {
		service.config = serviceCfg
	}
	return service
}

func (s *ImageService) logSafetyAudit(entry SafetyCheckAuditLog) {
	s.auditLogMutex.Lock()
	defer s.auditLogMutex.Unlock()
	
	entry.Timestamp = time.Now()
	s.auditLogs = append(s.auditLogs, entry)
	
	if len(s.auditLogs) > s.config.MaxAuditLogs {
		s.auditLogs = s.auditLogs[len(s.auditLogs)-s.config.MaxAuditLogs:]
	}
	
	log.Printf("[SAFETY AUDIT] work_order=%s, expert=%s, status=%s, is_safe=%v, warnings=%s",
		entry.WorkOrderID, entry.ExpertID, entry.CheckStatus, entry.IsSafe, entry.Warnings)
}

func (s *ImageService) checkCircuitBreaker() error {
	if !s.config.CircuitBreakerEnabled {
		return nil
	}
	
	s.circuitBreakerMutex.RLock()
	state := s.circuitState
	openTime := s.circuitOpenTime
	s.circuitBreakerMutex.RUnlock()
	
	if state == CircuitOpen {
		if time.Since(openTime) >= s.config.CircuitOpenDuration {
			s.circuitBreakerMutex.Lock()
			s.circuitState = CircuitHalfOpen
			s.circuitBreakerMutex.Unlock()
			log.Printf("[CIRCUIT BREAKER] Transitioning to HALF-OPEN state after cool-down period")
		} else {
			return errors.New("CIRCUIT BREAKER OPEN: Safety check service unavailable due to consecutive failures. Prescription BLOCKED.")
		}
	}
	
	return nil
}

func (s *ImageService) recordSuccess() {
	if !s.config.CircuitBreakerEnabled {
		return
	}
	
	s.circuitBreakerMutex.Lock()
	defer s.circuitBreakerMutex.Unlock()
	
	s.consecutiveFailures = 0
	if s.circuitState == CircuitHalfOpen {
		s.circuitState = CircuitClosed
		log.Printf("[CIRCUIT BREAKER] Transitioning to CLOSED state - service recovered")
	}
}

func (s *ImageService) recordFailure() {
	if !s.config.CircuitBreakerEnabled {
		return
	}
	
	s.circuitBreakerMutex.Lock()
	defer s.circuitBreakerMutex.Unlock()
	
	s.consecutiveFailures++
	
	if s.circuitState == CircuitHalfOpen {
		s.circuitState = CircuitOpen
		s.circuitOpenTime = time.Now()
		log.Printf("[CIRCUIT BREAKER] Transitioning to OPEN state from HALF-OPEN - service still failing")
		return
	}
	
	if s.consecutiveFailures >= s.config.CircuitFailureThreshold && s.circuitState == CircuitClosed {
		s.circuitState = CircuitOpen
		s.circuitOpenTime = time.Now()
		log.Printf("[CIRCUIT BREAKER] TRIPPED - %d consecutive failures, circuit OPEN for %v",
			s.consecutiveFailures, s.config.CircuitOpenDuration)
	}
}

func (s *ImageService) GetCircuitStatus() (CircuitBreakerState, int, time.Time) {
	s.circuitBreakerMutex.RLock()
	defer s.circuitBreakerMutex.RUnlock()
	return s.circuitState, s.consecutiveFailures, s.circuitOpenTime
}

func (s *ImageService) ResetCircuitBreaker() {
	s.circuitBreakerMutex.Lock()
	defer s.circuitBreakerMutex.Unlock()
	s.circuitState = CircuitClosed
	s.consecutiveFailures = 0
	log.Printf("[CIRCUIT BREAKER] Manually reset to CLOSED state")
}

func (s *ImageService) DiagnoseImageWithWorkOrderID(
	workOrderID string,
	imageData []byte,
	imageName string,
	cropType string,
	farmerID string,
) (*DiagnosisResponse, error) {
	if workOrderID == "" {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       errors.New("work_order_id is required for identity verification"),
		}
	}

	if farmerID == "" {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       errors.New("farmer_id is required for identity verification"),
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.config.DiagnosisTimeout)
	defer cancel()

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	imagePart, err := writer.CreateFormFile("image", imageName)
	if err != nil {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       err,
		}
	}
	_, err = imagePart.Write(imageData)
	if err != nil {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       err,
		}
	}

	writer.WriteField("crop_type", cropType)
	writer.WriteField("work_order_id", workOrderID)
	writer.WriteField("farmer_id", farmerID)
	writer.WriteField("request_time", time.Now().UTC().Format(time.RFC3339Nano))
	writer.Close()

	url := fmt.Sprintf("%s/api/diagnose", s.pythonServiceURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &requestBody)
	if err != nil {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       err,
		}
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("X-Work-Order-ID", workOrderID)
	req.Header.Set("X-Farmer-ID", farmerID)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		isTimeout := false
		if errors.Is(err, context.DeadlineExceeded) {
			isTimeout = true
		} else if netErr, ok := err.(net.Error); ok {
			isTimeout = netErr.Timeout()
		}
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       err,
			IsTimeout: isTimeout,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       err,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       fmt.Errorf("AI service returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	var result DiagnosisResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       fmt.Errorf("failed to parse diagnosis response: %v", err),
		}
	}

	if !result.Success {
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err:       errors.New("AI service returned unsuccessful diagnosis"),
		}
	}

	if result.WorkOrderID != workOrderID {
		log.Printf("[CRITICAL SECURITY ALERT] Diagnosis result work_order_id MISMATCH! Expected: %s, Got: %s, FarmerID: %s",
			workOrderID, result.WorkOrderID, farmerID)
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err: fmt.Errorf(
				"CRITICAL SECURITY: Diagnosis result work_order_id mismatch! Expected: %s, Got: %s",
				workOrderID, result.WorkOrderID,
			),
		}
	}

	log.Printf("[SECURE DIAGNOSIS] work_order_id=%s, farmer_id=%s, disease=%s, confidence=%.2f",
		workOrderID, farmerID, result.DiseaseName, result.Confidence)

	return &result, nil
}

func (s *ImageService) DiagnoseImage(imageData []byte, imageName string, cropType string) (*DiagnosisResponse, error) {
	log.Printf("[SECURITY VIOLATION ATTEMPT] Deprecated DiagnoseImage called without identity verification")
	return nil, &ServiceError{
		Operation: "DiagnoseImage",
		Err:       errors.New("deprecated: must use DiagnoseImageWithWorkOrderID with proper identity verification"),
	}
}

func (s *ImageService) CheckPrescriptionSafety(
	medications []string,
	workOrderID string,
	expertID string,
) (*PrescriptionSafetyResult, error) {
	auditEntry := SafetyCheckAuditLog{
		WorkOrderID: workOrderID,
		ExpertID:    expertID,
		Medications: medications,
		CheckStatus: SafetyCheckError,
		IsSafe:      false,
	}
	startTime := time.Now()
	
	defer func() {
		auditEntry.ResponseTime = time.Since(startTime)
		s.logSafetyAudit(auditEntry)
	}()

	if err := s.checkCircuitBreaker(); err != nil {
		auditEntry.CheckStatus = SafetyCheckCircuitOpen
		auditEntry.Warnings = "CIRCUIT BREAKER: Safety check service has been disabled due to consecutive failures"
		auditEntry.ErrorDetails = err.Error()
		
		return &PrescriptionSafetyResult{
			IsSafe:           false,
			Warnings:         "🚨 CRITICAL: Safety check service is DOWN due to consecutive failures.",
			Suggestions:      "Prescription issuance is BLOCKED. Please contact technical support to investigate the service.",
			Error:            err,
			IsFallback:       true,
			CheckStatus:      SafetyCheckCircuitOpen,
			CheckTimestamp:   time.Now(),
			ServiceAvailable: false,
		}, err
	}

	if workOrderID == "" || expertID == "" {
		auditEntry.Warnings = "Missing identity parameters for security verification"
		auditEntry.ErrorDetails = "work_order_id or expert_id is empty"
		
		return &PrescriptionSafetyResult{
			IsSafe:           false,
			Warnings:         "🚨 SECURITY ERROR: Missing identity verification parameters.",
			Suggestions:      "Prescription issuance BLOCKED. Please ensure work_order_id and expert_id are provided.",
			Error:            errors.New("security verification failed: missing identity parameters"),
			IsFallback:       true,
			CheckStatus:      SafetyCheckFailed,
			CheckTimestamp:   time.Now(),
			ServiceAvailable: true,
		}, errors.New("security verification failed: missing identity parameters")
	}

	if len(medications) == 0 {
		auditEntry.CheckStatus = SafetyCheckPassed
		auditEntry.IsSafe = true
		auditEntry.Warnings = "No medications specified - compatibility check skipped but verified as safe"
		
		log.Printf("[SAFETY CHECK] No medications specified for work_order=%s, expert=%s", workOrderID, expertID)
		
		return &PrescriptionSafetyResult{
			IsSafe:           true,
			Warnings:         "No medications specified. Single-agent use is permitted, but please follow label instructions.",
			Suggestions:      "Always verify dosage and application method before use.",
			Error:            nil,
			IsFallback:       false,
			CheckStatus:      SafetyCheckPassed,
			CheckTimestamp:   time.Now(),
			ServiceAvailable: true,
		}, nil
	}

	medicationsJSON, err := json.Marshal(medications)
	if err != nil {
		auditEntry.ErrorDetails = fmt.Sprintf("Failed to marshal medications: %v", err)
		
		return &PrescriptionSafetyResult{
			IsSafe:           false,
			Warnings:         "🚨 INTERNAL ERROR: Failed to process medication list.",
			Suggestions:      "Prescription BLOCKED. Please try again or contact support.",
			Error:            err,
			IsFallback:       true,
			CheckStatus:      SafetyCheckError,
			CheckTimestamp:   time.Now(),
			ServiceAvailable: true,
		}, err
	}

	var lastErr error
	var lastResp *PrescriptionCheckResponse
	var lastStatusCode int
	attempts := 0

	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		attempts = attempt + 1
		if attempt > 0 {
			sleepDuration := s.config.RetryInterval * time.Duration(attempt)
			log.Printf("[SAFETY CHECK] Retry %d/%d after %v...", attempts, s.config.MaxRetries, sleepDuration)
			time.Sleep(sleepDuration)
		}

		ctx, cancel := context.WithTimeout(context.Background(), s.config.PrescriptionTimeout)
		defer cancel()

		url := fmt.Sprintf("%s/api/check-prescription", s.pythonServiceURL)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(medicationsJSON))
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to create request: %v", attempts, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Work-Order-ID", workOrderID)
		req.Header.Set("X-Expert-ID", expertID)
		req.Header.Set("X-Attempt", fmt.Sprintf("%d", attempts))

		resp, err := s.httpClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				lastErr = fmt.Errorf("attempt %d: request TIMEOUT after %v", attempts, s.config.PrescriptionTimeout)
				auditEntry.CheckStatus = SafetyCheckTimeout
			} else if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				lastErr = fmt.Errorf("attempt %d: network TIMEOUT", attempts)
				auditEntry.CheckStatus = SafetyCheckTimeout
			} else {
				lastErr = fmt.Errorf("attempt %d: request failed: %v", attempts, err)
			}
			continue
		}
		defer resp.Body.Close()

		lastStatusCode = resp.StatusCode

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to read response: %v", attempts, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("attempt %d: service returned status %d: %s", attempts, resp.StatusCode, string(body))
			continue
		}

		var result PrescriptionCheckResponse
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("attempt %d: failed to parse response: %v", attempts, err)
			continue
		}

		lastResp = &result
		lastErr = nil
		break
	}

	auditEntry.Attempts = attempts

	if lastErr != nil || lastResp == nil {
		s.recordFailure()
		
		auditEntry.CheckStatus = SafetyCheckFailed
		auditEntry.ErrorDetails = lastErr.Error()
		auditEntry.IsSafe = false
		
		if auditEntry.CheckStatus == SafetyCheckTimeout {
			auditEntry.Warnings = fmt.Sprintf("Safety check TIMEOUT after %d attempts. Medications: %v", attempts, medications)
		} else {
			auditEntry.Warnings = fmt.Sprintf("Safety check FAILED after %d attempts. Last error: %v", attempts, lastErr)
		}

		log.Printf("[SAFETY CHECK FAILED] work_order=%s, expert=%s, attempts=%d, last_status=%d, error=%v",
			workOrderID, expertID, attempts, lastStatusCode, lastErr)

		if s.config.FailOpen {
			log.Printf("[SAFETY CHECK] FAIL-OPEN MODE ENABLED - Allowing prescription despite safety check failure (NOT RECOMMENDED)")
			auditEntry.IsSafe = true
			auditEntry.Warnings = "⚠️ FAIL-OPEN: Safety check failed but prescription allowed due to fail-open configuration"
			
			return &PrescriptionSafetyResult{
				IsSafe:      true,
				Warnings:    "⚠️ WARNING: Compatibility check service unavailable. Using FAIL-OPEN mode (NOT RECOMMENDED for production).",
				Suggestions: "Prescription allowed, but YOU MUST verify safety manually before issuing. This is a fallback mode and may be unsafe.",
				Error:       lastErr,
				IsFallback:  true,
				CheckStatus: auditEntry.CheckStatus,
				CheckTimestamp: time.Now(),
				ServiceAvailable: false,
			}, nil
		}

		return &PrescriptionSafetyResult{
			IsSafe:           false,
			Warnings:         "🚨 CRITICAL SAFETY ALERT: Prescription compatibility check FAILED or TIMED OUT.",
			Suggestions:      "Prescription issuance is BLOCKED for your safety. Please verify the medication combination manually or try again later.",
			Error:            fmt.Errorf("safety check failed after %d attempts: %v", attempts, lastErr),
			IsFallback:       true,
			CheckStatus:      auditEntry.CheckStatus,
			CheckTimestamp:   time.Now(),
			ServiceAvailable: false,
		}, fmt.Errorf("prescription safety check FAILED: %v", lastErr)
	}

	s.recordSuccess()
	auditEntry.CheckStatus = SafetyCheckPassed
	auditEntry.IsSafe = lastResp.IsSafe
	auditEntry.Warnings = lastResp.Warnings

	log.Printf("[SAFETY CHECK PASSED] work_order=%s, expert=%s, is_safe=%v, medications=%v",
		workOrderID, expertID, lastResp.IsSafe, medications)

	if !lastResp.IsSafe {
		log.Printf("[SAFETY CHECK] INCOMPATIBLE MEDICATIONS detected for work_order=%s: %s",
			workOrderID, lastResp.Warnings)
	}

	return &PrescriptionSafetyResult{
		IsSafe:           lastResp.IsSafe,
		Warnings:         lastResp.Warnings,
		Suggestions:      lastResp.Suggestions,
		Error:            nil,
		IsFallback:       false,
		CheckStatus:      SafetyCheckPassed,
		CheckTimestamp:   time.Now(),
		ServiceAvailable: true,
	}, nil
}

func (s *ImageService) CheckPrescriptionCompatibilityWithRetry(
	medications []string,
	workOrderID string,
	expertID string,
) (*PrescriptionSafetyResult, error) {
	return s.CheckPrescriptionSafety(medications, workOrderID, expertID)
}

func (s *ImageService) CheckPrescriptionCompatibility(medications []string) (*PrescriptionCheckResponse, error) {
	log.Printf("[SECURITY VIOLATION ATTEMPT] Deprecated CheckPrescriptionCompatibility called without identity verification")
	return nil, &ServiceError{
		Operation: "CheckPrescriptionCompatibility",
		Err:       errors.New("deprecated: must use CheckPrescriptionSafety with proper security verification"),
	}
}

func (s *ImageService) GetAuditLogs() []SafetyCheckAuditLog {
	s.auditLogMutex.Lock()
	defer s.auditLogMutex.Unlock()
	
	result := make([]SafetyCheckAuditLog, len(s.auditLogs))
	copy(result, s.auditLogs)
	return result
}

func (s *ImageService) GetSimilarCases(diseaseName string, symptoms string) (*SimilarCasesResponse, error) {
	requestData := map[string]string{
		"disease_name": diseaseName,
		"symptoms":     symptoms,
	}

	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/similar-cases", s.pythonServiceURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result SimilarCasesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *ImageService) GenerateTreatmentPlan(diseaseName string, severity string, cropType string) (string, error) {
	requestData := map[string]string{
		"disease_name": diseaseName,
		"severity":     severity,
		"crop_type":    cropType,
	}

	requestJSON, err := json.Marshal(requestData)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/generate-plan", s.pythonServiceURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestJSON))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if plan, ok := result["treatment_plan"].(string); ok {
		return plan, nil
	}

	return "", fmt.Errorf("unexpected response format")
}

func (s *ImageService) DiagnosisResultToModel(resp *DiagnosisResponse) *models.DiagnosisResult {
	return &models.DiagnosisResult{
		DiseaseName:       resp.DiseaseName,
		DiseaseType:       resp.DiseaseType,
		Confidence:        resp.Confidence,
		Symptoms:          resp.Symptoms,
		Causes:            resp.Causes,
		RecommendedActions: resp.RecommendedActions,
		Severity:          resp.Severity,
		SimilarCases:      resp.SimilarCases,
	}
}

func (s *ImageService) HealthCheck() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/health", s.pythonServiceURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
