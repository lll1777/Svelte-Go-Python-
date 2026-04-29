package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"agriculture-platform/config"
	"agriculture-platform/models"
)

type ImageService struct {
	pythonServiceURL string
	httpClient       *http.Client
}

type DiagnosisResponse struct {
	Success      bool    `json:"success"`
	DiseaseName  string  `json:"disease_name"`
	DiseaseType  string  `json:"disease_type"`
	Confidence   float64 `json:"confidence"`
	Symptoms     string  `json:"symptoms"`
	Causes       string  `json:"causes"`
	RecommendedActions string `json:"recommended_actions"`
	Severity     string    `json:"severity"`
	SimilarCases string    `json:"similar_cases"`
	ImageHash    string    `json:"image_hash"`
}

type PrescriptionCheckResponse struct {
	Success      bool   `json:"success"`
	IsSafe       bool   `json:"is_safe"`
	Warnings     string `json:"warnings"`
	Suggestions  string `json:"suggestions"`
}

type SimilarCasesResponse struct {
	Success bool   `json:"success"`
	Cases   string `json:"cases"`
}

func NewImageService(cfg *config.Config) *ImageService {
	return &ImageService{
		pythonServiceURL: cfg.PythonServiceURL,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (s *ImageService) DiagnoseImage(imageData []byte, imageName string, cropType string) (*DiagnosisResponse, error) {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	imagePart, err := writer.CreateFormFile("image", imageName)
	if err != nil {
		return nil, err
	}
	_, err = imagePart.Write(imageData)
	if err != nil {
		return nil, err
	}

	writer.WriteField("crop_type", cropType)
	writer.Close()

	url := fmt.Sprintf("%s/api/diagnose", s.pythonServiceURL)
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result DiagnosisResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *ImageService) CheckPrescriptionCompatibility(medications []string) (*PrescriptionCheckResponse, error) {
	medicationsJSON, err := json.Marshal(medications)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/api/check-prescription", s.pythonServiceURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(medicationsJSON))
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

	var result PrescriptionCheckResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
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

	url := fmt.Sprintf("%s/api/similar-cases", s.pythonServiceURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestJSON))
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

	url := fmt.Sprintf("%s/api/generate-plan", s.pythonServiceURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestJSON))
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
	url := fmt.Sprintf("%s/health", s.pythonServiceURL)
	resp, err := s.httpClient.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
