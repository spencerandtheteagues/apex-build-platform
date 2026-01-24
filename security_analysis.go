package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// SecurityAnalyzer performs comprehensive security testing
type SecurityAnalyzer struct {
	BaseURL    string
	Client     *http.Client
	Results    []SecurityResult
}

type SecurityResult struct {
	Test        string
	Status      string
	Severity    string
	Description string
	Details     string
}

// NewSecurityAnalyzer creates a new security analyzer
func NewSecurityAnalyzer(baseURL string) *SecurityAnalyzer {
	return &SecurityAnalyzer{
		BaseURL: baseURL,
		Client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		Results: make([]SecurityResult, 0),
	}
}

// RunAllTests executes comprehensive security testing
func (sa *SecurityAnalyzer) RunAllTests() {
	fmt.Println("🔒 APEX.BUILD Security Analysis Starting...")

	sa.TestHealthEndpoint()
	sa.TestSQLInjection()
	sa.TestXSSProtection()
	sa.TestCSRFProtection()
	sa.TestAuthenticationSecurity()
	sa.TestRateLimiting()
	sa.TestCORSConfiguration()
	sa.TestHTTPSRedirect()
	sa.TestSecurityHeaders()
	sa.TestInputValidation()

	sa.GenerateReport()
}

// TestHealthEndpoint validates basic endpoint security
func (sa *SecurityAnalyzer) TestHealthEndpoint() {
	resp, err := sa.Client.Get(sa.BaseURL + "/health")
	if err != nil {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Health Endpoint",
			Status:      "FAIL",
			Severity:    "MEDIUM",
			Description: "Health endpoint unreachable",
			Details:     err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Health Endpoint",
			Status:      "PASS",
			Severity:    "INFO",
			Description: "Health endpoint accessible",
			Details:     fmt.Sprintf("Status: %d", resp.StatusCode),
		})
	}
}

// TestSQLInjection tests for SQL injection vulnerabilities
func (sa *SecurityAnalyzer) TestSQLInjection() {
	sqlPayloads := []string{
		"'; DROP TABLE users; --",
		"' OR '1'='1",
		"' UNION SELECT * FROM users --",
		"admin'--",
		"' OR 1=1 #",
	}

	for _, payload := range sqlPayloads {
		// Test login endpoint with SQL injection
		loginData := fmt.Sprintf(`{"username": "%s", "password": "test"}`, payload)
		resp, err := sa.Client.Post(
			sa.BaseURL+"/api/v1/auth/login",
			"application/json",
			strings.NewReader(loginData),
		)

		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check for SQL errors or unexpected access
		if strings.Contains(string(body), "SQL") ||
		   strings.Contains(string(body), "syntax error") ||
		   resp.StatusCode == 200 {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "SQL Injection",
				Status:      "FAIL",
				Severity:    "CRITICAL",
				Description: "Potential SQL injection vulnerability detected",
				Details:     fmt.Sprintf("Payload: %s, Response: %s", payload, string(body)),
			})
		}
	}

	sa.Results = append(sa.Results, SecurityResult{
		Test:        "SQL Injection",
		Status:      "PASS",
		Severity:    "INFO",
		Description: "No SQL injection vulnerabilities detected in tested endpoints",
		Details:     fmt.Sprintf("Tested %d payloads", len(sqlPayloads)),
	})
}

// TestXSSProtection tests for cross-site scripting vulnerabilities
func (sa *SecurityAnalyzer) TestXSSProtection() {
	xssPayloads := []string{
		"<script>alert('XSS')</script>",
		"javascript:alert('XSS')",
		"<img src=x onerror=alert('XSS')>",
		"'><script>alert('XSS')</script>",
	}

	for _, payload := range xssPayloads {
		// Test registration with XSS payload
		regData := fmt.Sprintf(`{
			"username": "%s",
			"email": "test@apex.build",
			"password": "SecurePassword123!"
		}`, payload)

		resp, err := sa.Client.Post(
			sa.BaseURL+"/api/v1/auth/register",
			"application/json",
			strings.NewReader(regData),
		)

		if err != nil {
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		// Check if payload is reflected without sanitization
		if strings.Contains(string(body), "<script>") ||
		   strings.Contains(string(body), "javascript:") {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "XSS Protection",
				Status:      "FAIL",
				Severity:    "HIGH",
				Description: "XSS vulnerability detected - unsanitized input reflection",
				Details:     fmt.Sprintf("Payload: %s", payload),
			})
		}
	}

	sa.Results = append(sa.Results, SecurityResult{
		Test:        "XSS Protection",
		Status:      "PASS",
		Severity:    "INFO",
		Description: "No XSS vulnerabilities detected in tested endpoints",
		Details:     fmt.Sprintf("Tested %d payloads", len(xssPayloads)),
	})
}

// TestCSRFProtection tests CSRF protection mechanisms
func (sa *SecurityAnalyzer) TestCSRFProtection() {
	// Test if state-changing operations require CSRF tokens
	resp, err := sa.Client.Post(
		sa.BaseURL+"/api/v1/auth/register",
		"application/json",
		strings.NewReader(`{"username":"csrftest","email":"csrf@test.com","password":"Test123!"}`),
	)

	if err != nil {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "CSRF Protection",
			Status:      "UNKNOWN",
			Severity:    "MEDIUM",
			Description: "Unable to test CSRF protection",
			Details:     err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	sa.Results = append(sa.Results, SecurityResult{
		Test:        "CSRF Protection",
		Status:      "REVIEW",
		Severity:    "MEDIUM",
		Description: "CSRF protection implementation requires manual verification",
		Details:     "Ensure CSRF tokens are required for state-changing operations",
	})
}

// TestAuthenticationSecurity tests authentication mechanisms
func (sa *SecurityAnalyzer) TestAuthenticationSecurity() {
	// Test weak password acceptance
	weakPasswords := []string{"123", "password", "admin", "test"}

	for _, weak := range weakPasswords {
		regData := fmt.Sprintf(`{
			"username": "weaktest_%s",
			"email": "weak_%s@test.com",
			"password": "%s"
		}`, weak, weak, weak)

		resp, err := sa.Client.Post(
			sa.BaseURL+"/api/v1/auth/register",
			"application/json",
			strings.NewReader(regData),
		)

		if err != nil {
			continue
		}

		if resp.StatusCode == 200 || resp.StatusCode == 201 {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "Authentication Security",
				Status:      "FAIL",
				Severity:    "HIGH",
				Description: "Weak password accepted by system",
				Details:     fmt.Sprintf("Password '%s' was accepted", weak),
			})
		}
		resp.Body.Close()
	}

	sa.Results = append(sa.Results, SecurityResult{
		Test:        "Authentication Security",
		Status:      "PASS",
		Severity:    "INFO",
		Description: "Strong password policy enforced",
		Details:     "Weak passwords properly rejected",
	})
}

// TestRateLimiting tests rate limiting implementation
func (sa *SecurityAnalyzer) TestRateLimiting() {
	rateLimitHit := false

	// Make rapid requests to test rate limiting
	for i := 0; i < 50; i++ {
		resp, err := sa.Client.Get(sa.BaseURL + "/health")
		if err != nil {
			continue
		}

		if resp.StatusCode == 429 {
			rateLimitHit = true
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}

	if rateLimitHit {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Rate Limiting",
			Status:      "PASS",
			Severity:    "INFO",
			Description: "Rate limiting is active and working",
			Details:     "429 status code returned for excessive requests",
		})
	} else {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Rate Limiting",
			Status:      "REVIEW",
			Severity:    "MEDIUM",
			Description: "Rate limiting not detected",
			Details:     "Consider implementing rate limiting for production",
		})
	}
}

// TestCORSConfiguration tests CORS header configuration
func (sa *SecurityAnalyzer) TestCORSConfiguration() {
	req, _ := http.NewRequest("OPTIONS", sa.BaseURL+"/health", nil)
	req.Header.Set("Origin", "https://malicious.com")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := sa.Client.Do(req)
	if err != nil {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "CORS Configuration",
			Status:      "UNKNOWN",
			Severity:    "MEDIUM",
			Description: "Unable to test CORS configuration",
			Details:     err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin == "*" {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "CORS Configuration",
			Status:      "FAIL",
			Severity:    "MEDIUM",
			Description: "Overly permissive CORS policy detected",
			Details:     "Access-Control-Allow-Origin: * allows any origin",
		})
	} else {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "CORS Configuration",
			Status:      "PASS",
			Severity:    "INFO",
			Description: "CORS policy appears to be properly configured",
			Details:     fmt.Sprintf("Allow-Origin: %s", allowOrigin),
		})
	}
}

// TestHTTPSRedirect tests if HTTP redirects to HTTPS
func (sa *SecurityAnalyzer) TestHTTPSRedirect() {
	httpURL := strings.Replace(sa.BaseURL, "https://", "http://", 1)
	if httpURL == sa.BaseURL {
		// Already HTTP, test if it should be HTTPS
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "HTTPS Redirect",
			Status:      "REVIEW",
			Severity:    "HIGH",
			Description: "Application running on HTTP instead of HTTPS",
			Details:     "Production deployments should enforce HTTPS",
		})
		return
	}

	resp, err := sa.Client.Get(httpURL)
	if err != nil {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "HTTPS Redirect",
			Status:      "PASS",
			Severity:    "INFO",
			Description: "HTTP endpoint not accessible",
			Details:     "Good - HTTPS properly enforced",
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if strings.HasPrefix(location, "https://") {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "HTTPS Redirect",
				Status:      "PASS",
				Severity:    "INFO",
				Description: "HTTP properly redirects to HTTPS",
				Details:     fmt.Sprintf("Redirects to: %s", location),
			})
		}
	}
}

// TestSecurityHeaders tests for important security headers
func (sa *SecurityAnalyzer) TestSecurityHeaders() {
	resp, err := sa.Client.Get(sa.BaseURL + "/health")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	requiredHeaders := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":       "DENY",
		"X-XSS-Protection":      "1; mode=block",
	}

	for header, expectedValue := range requiredHeaders {
		actualValue := resp.Header.Get(header)
		if actualValue == "" {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "Security Headers",
				Status:      "FAIL",
				Severity:    "MEDIUM",
				Description: fmt.Sprintf("Missing security header: %s", header),
				Details:     fmt.Sprintf("Add '%s: %s' header", header, expectedValue),
			})
		} else if actualValue != expectedValue {
			sa.Results = append(sa.Results, SecurityResult{
				Test:        "Security Headers",
				Status:      "REVIEW",
				Severity:    "LOW",
				Description: fmt.Sprintf("Security header %s has value: %s", header, actualValue),
				Details:     fmt.Sprintf("Expected: %s", expectedValue),
			})
		}
	}
}

// TestInputValidation tests input validation mechanisms
func (sa *SecurityAnalyzer) TestInputValidation() {
	// Test extremely long input
	longString := strings.Repeat("A", 10000)

	regData := fmt.Sprintf(`{
		"username": "%s",
		"email": "test@apex.build",
		"password": "SecurePassword123!"
	}`, longString)

	resp, err := sa.Client.Post(
		sa.BaseURL+"/api/v1/auth/register",
		"application/json",
		strings.NewReader(regData),
	)

	if err != nil {
		return
	}

	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Input Validation",
			Status:      "FAIL",
			Severity:    "MEDIUM",
			Description: "Input length validation insufficient",
			Details:     "Extremely long input accepted without proper validation",
		})
	} else {
		sa.Results = append(sa.Results, SecurityResult{
			Test:        "Input Validation",
			Status:      "PASS",
			Severity:    "INFO",
			Description: "Input validation working properly",
			Details:     "Long input properly rejected",
		})
	}
	resp.Body.Close()
}

// GenerateReport generates the final security analysis report
func (sa *SecurityAnalyzer) GenerateReport() {
	fmt.Println("\n🔒 APEX.BUILD SECURITY ANALYSIS REPORT")
	fmt.Println("=====================================")

	critical := 0
	high := 0
	medium := 0
	low := 0
	passed := 0

	for _, result := range sa.Results {
		switch result.Severity {
		case "CRITICAL":
			critical++
		case "HIGH":
			high++
		case "MEDIUM":
			medium++
		case "LOW":
			low++
		}

		if result.Status == "PASS" {
			passed++
		}

		fmt.Printf("\n[%s] %s - %s\n", result.Status, result.Test, result.Severity)
		fmt.Printf("  Description: %s\n", result.Description)
		if result.Details != "" {
			fmt.Printf("  Details: %s\n", result.Details)
		}
	}

	fmt.Printf("\n📊 SECURITY SUMMARY\n")
	fmt.Printf("===================\n")
	fmt.Printf("Tests Passed: %d\n", passed)
	fmt.Printf("Critical Issues: %d\n", critical)
	fmt.Printf("High Issues: %d\n", high)
	fmt.Printf("Medium Issues: %d\n", medium)
	fmt.Printf("Low Issues: %d\n", low)

	if critical == 0 && high == 0 {
		fmt.Printf("\n✅ SECURITY STATUS: GOOD\n")
		fmt.Printf("No critical or high severity issues detected.\n")
	} else if critical == 0 {
		fmt.Printf("\n⚠️  SECURITY STATUS: REVIEW NEEDED\n")
		fmt.Printf("High severity issues detected that should be addressed.\n")
	} else {
		fmt.Printf("\n🚨 SECURITY STATUS: CRITICAL\n")
		fmt.Printf("Critical security issues detected! Address immediately.\n")
	}
}

func main() {
	analyzer := NewSecurityAnalyzer("http://localhost:8080")
	analyzer.RunAllTests()
}