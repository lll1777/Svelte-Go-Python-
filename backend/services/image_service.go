package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"time"

	"agriculture-platform/config"
	"agriculture-platform/models"
)

type ImageService struct {
	pythonServiceURL string
	httpClient       *http.Client
	config           *ServiceConfig
}

type ServiceConfig struct {
	DiagnosisTimeout     time.Duration
	PrescriptionTimeout  time.Duration
	MaxRetries           int
	RetryInterval        time.Duration
	FailOpen             bool
}

var DefaultServiceConfig = &ServiceConfig{
	DiagnosisTimeout:    30 * time.Second,
	PrescriptionTimeout: 15 * time.Second,
	MaxRetries:          3,
	RetryInterval:       2 * time.Second,
	FailOpen:            false,
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
	}
}

func NewImageServiceWithConfig(cfg *config.Config, serviceCfg *ServiceConfig) *ImageService {
	service := NewImageService(cfg)
	if serviceCfg != nil {
		service.config = serviceCfg
	}
	return service
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
		return nil, &ServiceError{
			Operation: "DiagnoseImage",
			Err: fmt.Errorf(
				"CRITICAL SECURITY: Diagnosis result work_order_id mismatch! Expected: %s, Got: %s",
				workOrderID, result.WorkOrderID,
			),
		}
	}

	return &result, nil
}

func (s *ImageService) DiagnoseImage(imageData []byte, imageName string, cropType string) (*DiagnosisResponse, error) {
	return nil, &ServiceError{
		Operation: "DiagnoseImage",
		Err:       errors.New("deprecated: must use DiagnoseImageWithWorkOrderID with proper identity verification"),
	}
}

type PrescriptionCheckResult struct {
	IsSafe      bool
	Warnings    string
	Suggestions string
	Error       error
	IsFallback  bool
}

func (s *ImageService) CheckPrescriptionCompatibilityWithRetry(
	medications []string,
	workOrderID string,
	expertID string,
) (*PrescriptionCheckResult, error) {
	if len(medications) == 0 {
		return &PrescriptionCheckResult{
			IsSafe:      true,
			Warnings:    "No medications specified, skipping compatibility check",
			Suggestions: "",
		}, nil
	}

	if workOrderID == "" || expertID == "" {
		return &PrescriptionCheckResult{
			IsSafe:     false,
			Error:      errors.New("work_order_id and expert_id are required for security verification"),
			IsFallback: true,
		}, errors.New("security verification failed: missing identity parameters")
	}

	medicationsJSON, err := json.Marshal(medications)
	if err != nil {
		return &PrescriptionCheckResult{
			IsSafe:     false,
			Error:      err,
			IsFallback: true,
		}, err
	}

	var lastErr error
	var lastResp *PrescriptionCheckResponse

	for attempt := 0; attempt < s.config.MaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(s.config.RetryInterval * time.Duration(attempt))
		}

		ctx, cancel := context.WithTimeout(context.Background(), s.config.PrescriptionTimeout)
		defer cancel()

		url := fmt.Sprintf("%s/api/check-prescription", s.pythonServiceURL)
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(medicationsJSON))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Work-Order-ID", workOrderID)
		req.Header.Set("X-Expert-ID", expertID)
		req.Header.Set("X-Attempt", fmt.Sprintf("%d", attempt+1))

		resp, err := s.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("service returned status %d: %s", resp.StatusCode, string(body))
			continue
		}

		var result PrescriptionCheckResponse
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("failed to parse response: %v", err)
			continue
		}

		lastResp = &result
		lastErr = nil
		break
	}

	if lastErr != nil || lastResp == nil {
		if s.config.FailOpen {
			return &PrescriptionCheckResult{
				IsSafe:      true,
				Warnings:    "WARNING: Compatibility check service unavailable. Using fail-open mode - proceed with caution!",
				Suggestions: "Please verify prescription safety manually before issuing.",
				Error:       lastErr,
				IsFallback:  true,
			}, nil
		}

		return &PrescriptionCheckResult{
			IsSafe:      false,
			Warnings:    "CRITICAL: Prescription compatibility check failed. Service unavailable or timed out.",
			Suggestions: "Prescription issuance BLOCKED for safety. Please try again or contact technical support.",
			Error:       lastErr,
			IsFallback:  true,
		}, fmt.Errorf("prescription safety check failed: %v", lastErr)
	}

	return &PrescriptionCheckResult{
		IsSafe:      lastResp.IsSafe,
		Warnings:    lastResp.Warnings,
		Suggestions: lastResp.Suggestions,
		IsFallback:  false,
	}, nil
}

func (s *ImageService) CheckPrescriptionCompatibility(medications []string) (*PrescriptionCheckResponse, error) {
	return nil, &ServiceError{
		Operation: "CheckPrescriptionCompatibility",
		Err:       errors.New("deprecated: must use CheckPrescriptionCompatibilityWithRetry with proper security verification"),
	}
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
