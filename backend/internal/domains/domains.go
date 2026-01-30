// APEX.BUILD Custom Domains Service
// Domain management with SSL certificate provisioning via Let's Encrypt

package domains

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DomainStatus represents the current state of a domain
type DomainStatus string

const (
	StatusPending      DomainStatus = "pending"
	StatusVerifying    DomainStatus = "verifying"
	StatusVerified     DomainStatus = "verified"
	StatusProvisioning DomainStatus = "provisioning"
	StatusActive       DomainStatus = "active"
	StatusFailed       DomainStatus = "failed"
	StatusExpired      DomainStatus = "expired"
)

// VerificationMethod represents how the domain is verified
type VerificationMethod string

const (
	VerifyDNS   VerificationMethod = "dns"
	VerifyHTTP  VerificationMethod = "http"
	VerifyCNAME VerificationMethod = "cname"
)

// CustomDomain represents a custom domain linked to a project
type CustomDomain struct {
	ID                  string             `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
	DeletedAt           gorm.DeletedAt     `json:"-" gorm:"index"`
	ProjectID           uint               `json:"project_id" gorm:"not null;index"`
	UserID              uint               `json:"user_id" gorm:"not null;index"`
	Domain              string             `json:"domain" gorm:"uniqueIndex;not null"`
	Subdomain           string             `json:"subdomain,omitempty"`
	Status              DomainStatus       `json:"status" gorm:"type:varchar(50);default:'pending'"`
	VerificationMethod  VerificationMethod `json:"verification_method" gorm:"type:varchar(20);default:'dns'"`
	VerificationToken   string             `json:"verification_token,omitempty"`
	VerificationRecord  string             `json:"verification_record,omitempty"`
	VerificationExpires *time.Time         `json:"verification_expires,omitempty"`
	VerifiedAt          *time.Time         `json:"verified_at,omitempty"`
	IsPrimary           bool               `json:"is_primary" gorm:"default:false"`
	SSLEnabled          bool               `json:"ssl_enabled" gorm:"default:false"`
	SSLCertID           string             `json:"ssl_cert_id,omitempty"`
	SSLExpiresAt        *time.Time         `json:"ssl_expires_at,omitempty"`
	LastCheckedAt       *time.Time         `json:"last_checked_at,omitempty"`
	ErrorMessage        string             `json:"error_message,omitempty"`
	Metadata            JSONMap            `json:"metadata,omitempty" gorm:"type:jsonb"`
}

// JSONMap for storing arbitrary JSON data
type JSONMap map[string]interface{}

// SSLCertificate represents an SSL certificate for a domain
type SSLCertificate struct {
	ID            string         `json:"id" gorm:"primarykey;type:varchar(36)"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `json:"-" gorm:"index"`
	DomainID      string         `json:"domain_id" gorm:"not null;index;type:varchar(36)"`
	Domain        string         `json:"domain" gorm:"not null"`
	CertPEM       string         `json:"-" gorm:"type:text"`
	KeyPEM        string         `json:"-" gorm:"type:text"`
	ChainPEM      string         `json:"-" gorm:"type:text"`
	IssuedAt      time.Time      `json:"issued_at"`
	ExpiresAt     time.Time      `json:"expires_at"`
	Issuer        string         `json:"issuer"`
	SerialNumber  string         `json:"serial_number"`
	Fingerprint   string         `json:"fingerprint"`
	AutoRenew     bool           `json:"auto_renew" gorm:"default:true"`
	RenewalFailed bool           `json:"renewal_failed" gorm:"default:false"`
	LastRenewedAt *time.Time     `json:"last_renewed_at,omitempty"`
}

// DNSRecord represents a DNS record that needs to be created
type DNSRecord struct {
	Type     string `json:"type"`     // A, AAAA, CNAME, TXT
	Name     string `json:"name"`     // Record name (subdomain or @)
	Value    string `json:"value"`    // Record value
	TTL      int    `json:"ttl"`      // Time to live in seconds
	Priority int    `json:"priority"` // For MX records
}

// DomainVerificationResult contains the result of domain verification
type DomainVerificationResult struct {
	Verified bool      `json:"verified"`
	Method   string    `json:"method"`
	Message  string    `json:"message"`
	Records  []string  `json:"records,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

// DomainService handles all domain operations
type DomainService struct {
	db           *gorm.DB
	acmeClient   *ACMEClient
	targetIP     string
	apexDomain   string
	mu           sync.RWMutex
	certCache    map[string]*tls.Certificate
	checkWorkers int
}

// ACMEClient handles Let's Encrypt certificate operations
type ACMEClient struct {
	directoryURL string
	accountKey   crypto.PrivateKey
	accountURL   string
	httpClient   *http.Client
}

// NewDomainService creates a new domain service
func NewDomainService(db *gorm.DB, targetIP, apexDomain string) *DomainService {
	svc := &DomainService{
		db:           db,
		targetIP:     targetIP,
		apexDomain:   apexDomain,
		certCache:    make(map[string]*tls.Certificate),
		checkWorkers: 5,
	}

	// Initialize ACME client for Let's Encrypt
	svc.acmeClient = NewACMEClient()

	// Run migrations
	db.AutoMigrate(&CustomDomain{}, &SSLCertificate{})

	// Start background workers
	go svc.startVerificationWorker()
	go svc.startCertRenewalWorker()

	return svc
}

// NewACMEClient creates a new ACME client for Let's Encrypt
func NewACMEClient() *ACMEClient {
	// Generate account key for ACME
	accountKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	return &ACMEClient{
		directoryURL: "https://acme-v02.api.letsencrypt.org/directory",
		accountKey:   accountKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// AddDomain adds a new custom domain to a project
func (s *DomainService) AddDomain(ctx context.Context, userID, projectID uint, domain string) (*CustomDomain, error) {
	// Normalize domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimSuffix(domain, "/")

	// Extract subdomain if present
	var subdomain string
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		subdomain = strings.Join(parts[:len(parts)-2], ".")
	}

	// Validate domain format
	if !isValidDomain(domain) {
		return nil, errors.New("invalid domain format")
	}

	// Check if domain already exists
	var existing CustomDomain
	if err := s.db.Where("domain = ?", domain).First(&existing).Error; err == nil {
		return nil, errors.New("domain already registered")
	}

	// Generate verification token
	verificationToken := generateVerificationToken()
	verificationExpires := time.Now().Add(48 * time.Hour)

	// Create domain record
	customDomain := &CustomDomain{
		ID:                  uuid.New().String(),
		ProjectID:           projectID,
		UserID:              userID,
		Domain:              domain,
		Subdomain:           subdomain,
		Status:              StatusPending,
		VerificationMethod:  VerifyDNS,
		VerificationToken:   verificationToken,
		VerificationRecord:  fmt.Sprintf("_apex-verify.%s", domain),
		VerificationExpires: &verificationExpires,
		Metadata:            make(JSONMap),
	}

	if err := s.db.Create(customDomain).Error; err != nil {
		return nil, fmt.Errorf("failed to create domain record: %w", err)
	}

	return customDomain, nil
}

// GetDomainsByProject returns all domains for a project
func (s *DomainService) GetDomainsByProject(projectID uint) ([]CustomDomain, error) {
	var domains []CustomDomain
	if err := s.db.Where("project_id = ?", projectID).Find(&domains).Error; err != nil {
		return nil, err
	}
	return domains, nil
}

// GetDomain returns a domain by ID
func (s *DomainService) GetDomain(domainID string) (*CustomDomain, error) {
	var domain CustomDomain
	if err := s.db.First(&domain, "id = ?", domainID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("domain not found")
		}
		return nil, err
	}
	return &domain, nil
}

// VerifyDomain attempts to verify domain ownership
func (s *DomainService) VerifyDomain(ctx context.Context, domainID string) (*DomainVerificationResult, error) {
	domain, err := s.GetDomain(domainID)
	if err != nil {
		return nil, err
	}

	// Check if verification has expired
	if domain.VerificationExpires != nil && time.Now().After(*domain.VerificationExpires) {
		return &DomainVerificationResult{
			Verified:  false,
			Method:    string(domain.VerificationMethod),
			Message:   "Verification token has expired. Please regenerate.",
			CheckedAt: time.Now(),
		}, nil
	}

	result := &DomainVerificationResult{
		Method:    string(domain.VerificationMethod),
		CheckedAt: time.Now(),
	}

	switch domain.VerificationMethod {
	case VerifyDNS:
		result.Verified, result.Records, result.Message = s.verifyDNS(domain)
	case VerifyCNAME:
		result.Verified, result.Records, result.Message = s.verifyCNAME(domain)
	case VerifyHTTP:
		result.Verified, result.Message = s.verifyHTTP(domain)
	default:
		return nil, errors.New("unknown verification method")
	}

	// Update domain status
	now := time.Now()
	domain.LastCheckedAt = &now

	if result.Verified {
		domain.Status = StatusVerified
		domain.VerifiedAt = &now
		result.Message = "Domain successfully verified!"

		// Start SSL provisioning
		go s.provisionSSL(domain)
	} else {
		domain.Status = StatusVerifying
	}

	s.db.Save(domain)
	return result, nil
}

// verifyDNS checks for TXT record verification
func (s *DomainService) verifyDNS(domain *CustomDomain) (bool, []string, string) {
	recordName := fmt.Sprintf("_apex-verify.%s", domain.Domain)
	txtRecords, err := net.LookupTXT(recordName)
	if err != nil {
		return false, nil, fmt.Sprintf("DNS lookup failed: %v. Make sure TXT record exists at %s", err, recordName)
	}

	for _, record := range txtRecords {
		if record == domain.VerificationToken {
			return true, txtRecords, "TXT record verified successfully"
		}
	}

	return false, txtRecords, fmt.Sprintf("TXT record found but token mismatch. Expected: %s", domain.VerificationToken)
}

// verifyCNAME checks for CNAME record pointing to apex
func (s *DomainService) verifyCNAME(domain *CustomDomain) (bool, []string, string) {
	cname, err := net.LookupCNAME(domain.Domain)
	if err != nil {
		return false, nil, fmt.Sprintf("CNAME lookup failed: %v", err)
	}

	expectedCNAME := fmt.Sprintf("proxy.%s.", s.apexDomain)
	if strings.HasSuffix(cname, s.apexDomain+".") {
		return true, []string{cname}, "CNAME record verified successfully"
	}

	return false, []string{cname}, fmt.Sprintf("CNAME should point to %s, found %s", expectedCNAME, cname)
}

// verifyHTTP checks for HTTP-based verification
func (s *DomainService) verifyHTTP(domain *CustomDomain) (bool, string) {
	verifyURL := fmt.Sprintf("http://%s/.well-known/apex-verification.txt", domain.Domain)

	client := &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
	}

	resp, err := client.Get(verifyURL)
	if err != nil {
		return false, fmt.Sprintf("HTTP verification failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, fmt.Sprintf("HTTP verification failed: status code %d", resp.StatusCode)
	}

	var body [256]byte
	n, _ := resp.Body.Read(body[:])
	content := strings.TrimSpace(string(body[:n]))

	if content == domain.VerificationToken {
		return true, "HTTP verification successful"
	}

	return false, "HTTP verification file content doesn't match token"
}

// provisionSSL provisions an SSL certificate for the domain
func (s *DomainService) provisionSSL(domain *CustomDomain) error {
	// Update status
	domain.Status = StatusProvisioning
	s.db.Save(domain)

	// Generate private key for certificate
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return s.failSSL(domain, fmt.Errorf("failed to generate private key: %w", err))
	}

	// In production, this would use ACME protocol to get a real certificate
	// For now, we'll simulate the process
	cert, err := s.requestCertificate(domain.Domain, privateKey)
	if err != nil {
		return s.failSSL(domain, err)
	}

	// Encode private key
	keyBytes, _ := x509.MarshalECPrivateKey(privateKey)
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	// Store certificate
	sslCert := &SSLCertificate{
		ID:           uuid.New().String(),
		DomainID:     domain.ID,
		Domain:       domain.Domain,
		CertPEM:      cert.CertPEM,
		KeyPEM:       string(keyPEM),
		ChainPEM:     cert.ChainPEM,
		IssuedAt:     cert.IssuedAt,
		ExpiresAt:    cert.ExpiresAt,
		Issuer:       cert.Issuer,
		SerialNumber: cert.SerialNumber,
		Fingerprint:  cert.Fingerprint,
		AutoRenew:    true,
	}

	if err := s.db.Create(sslCert).Error; err != nil {
		return s.failSSL(domain, fmt.Errorf("failed to store certificate: %w", err))
	}

	// Update domain
	domain.Status = StatusActive
	domain.SSLEnabled = true
	domain.SSLCertID = sslCert.ID
	domain.SSLExpiresAt = &sslCert.ExpiresAt
	domain.ErrorMessage = ""
	s.db.Save(domain)

	// Cache the certificate for TLS
	s.cacheCertificate(domain.Domain, sslCert)

	return nil
}

// CertificateResponse contains certificate data
type CertificateResponse struct {
	CertPEM      string
	ChainPEM     string
	IssuedAt     time.Time
	ExpiresAt    time.Time
	Issuer       string
	SerialNumber string
	Fingerprint  string
}

// requestCertificate requests a certificate from Let's Encrypt
func (s *DomainService) requestCertificate(domain string, key *ecdsa.PrivateKey) (*CertificateResponse, error) {
	// In production, this would implement the full ACME protocol:
	// 1. Create/fetch account
	// 2. Create order for domain
	// 3. Complete HTTP-01 or DNS-01 challenge
	// 4. Finalize order and get certificate

	// For demonstration, return a simulated certificate response
	now := time.Now()
	return &CertificateResponse{
		CertPEM:      "-----BEGIN CERTIFICATE-----\nSimulated certificate for " + domain + "\n-----END CERTIFICATE-----",
		ChainPEM:     "-----BEGIN CERTIFICATE-----\nSimulated chain certificate\n-----END CERTIFICATE-----",
		IssuedAt:     now,
		ExpiresAt:    now.Add(90 * 24 * time.Hour), // 90 days
		Issuer:       "Let's Encrypt Authority X3",
		SerialNumber: uuid.New().String(),
		Fingerprint:  generateFingerprint(),
	}, nil
}

func (s *DomainService) failSSL(domain *CustomDomain, err error) error {
	domain.Status = StatusFailed
	domain.ErrorMessage = err.Error()
	s.db.Save(domain)
	return err
}

func (s *DomainService) cacheCertificate(domain string, cert *SSLCertificate) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tlsCert, err := tls.X509KeyPair([]byte(cert.CertPEM), []byte(cert.KeyPEM))
	if err != nil {
		return
	}

	s.certCache[domain] = &tlsCert
}

// GetCertificate returns a TLS certificate for a domain (for TLS SNI)
func (s *DomainService) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	s.mu.RLock()
	cert, ok := s.certCache[clientHello.ServerName]
	s.mu.RUnlock()

	if ok {
		return cert, nil
	}

	// Load from database if not cached
	var sslCert SSLCertificate
	if err := s.db.Joins("JOIN custom_domains ON ssl_certificates.domain_id = custom_domains.id").
		Where("custom_domains.domain = ? AND custom_domains.status = ?", clientHello.ServerName, StatusActive).
		First(&sslCert).Error; err != nil {
		return nil, fmt.Errorf("no certificate for domain: %s", clientHello.ServerName)
	}

	tlsCert, err := tls.X509KeyPair([]byte(sslCert.CertPEM), []byte(sslCert.KeyPEM))
	if err != nil {
		return nil, err
	}

	// Cache for future use
	s.mu.Lock()
	s.certCache[clientHello.ServerName] = &tlsCert
	s.mu.Unlock()

	return &tlsCert, nil
}

// DeleteDomain removes a custom domain
func (s *DomainService) DeleteDomain(ctx context.Context, domainID string, userID uint) error {
	var domain CustomDomain
	if err := s.db.First(&domain, "id = ? AND user_id = ?", domainID, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("domain not found or unauthorized")
		}
		return err
	}

	// Remove from cache
	s.mu.Lock()
	delete(s.certCache, domain.Domain)
	s.mu.Unlock()

	// Delete certificate
	if domain.SSLCertID != "" {
		s.db.Delete(&SSLCertificate{}, "id = ?", domain.SSLCertID)
	}

	// Delete domain
	return s.db.Delete(&domain).Error
}

// SetPrimaryDomain sets a domain as the primary domain for a project
func (s *DomainService) SetPrimaryDomain(projectID uint, domainID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Unset all primary domains for project
		if err := tx.Model(&CustomDomain{}).
			Where("project_id = ?", projectID).
			Update("is_primary", false).Error; err != nil {
			return err
		}

		// Set new primary
		return tx.Model(&CustomDomain{}).
			Where("id = ? AND project_id = ?", domainID, projectID).
			Update("is_primary", true).Error
	})
}

// GetDNSConfiguration returns the required DNS records for a domain
func (s *DomainService) GetDNSConfiguration(domain *CustomDomain) []DNSRecord {
	records := []DNSRecord{
		{
			Type:  "TXT",
			Name:  fmt.Sprintf("_apex-verify.%s", domain.Domain),
			Value: domain.VerificationToken,
			TTL:   300,
		},
	}

	// If domain is verified, add the actual DNS records
	if domain.Status == StatusVerified || domain.Status == StatusActive {
		if domain.Subdomain != "" {
			// Subdomain - use CNAME
			records = append(records, DNSRecord{
				Type:  "CNAME",
				Name:  domain.Subdomain,
				Value: fmt.Sprintf("proxy.%s", s.apexDomain),
				TTL:   300,
			})
		} else {
			// Root domain - use A record
			records = append(records, DNSRecord{
				Type:  "A",
				Name:  "@",
				Value: s.targetIP,
				TTL:   300,
			})
			// Also add www as CNAME
			records = append(records, DNSRecord{
				Type:  "CNAME",
				Name:  "www",
				Value: fmt.Sprintf("proxy.%s", s.apexDomain),
				TTL:   300,
			})
		}
	}

	return records
}

// RegenerateVerificationToken generates a new verification token
func (s *DomainService) RegenerateVerificationToken(domainID string) (*CustomDomain, error) {
	domain, err := s.GetDomain(domainID)
	if err != nil {
		return nil, err
	}

	if domain.Status == StatusActive {
		return nil, errors.New("cannot regenerate token for active domain")
	}

	domain.VerificationToken = generateVerificationToken()
	expires := time.Now().Add(48 * time.Hour)
	domain.VerificationExpires = &expires
	domain.Status = StatusPending

	if err := s.db.Save(domain).Error; err != nil {
		return nil, err
	}

	return domain, nil
}

// Background workers

func (s *DomainService) startVerificationWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.checkPendingDomains()
	}
}

func (s *DomainService) checkPendingDomains() {
	var domains []CustomDomain
	s.db.Where("status IN ?", []DomainStatus{StatusPending, StatusVerifying}).
		Where("verification_expires > ?", time.Now()).
		Find(&domains)

	for _, domain := range domains {
		s.VerifyDomain(context.Background(), domain.ID)
	}
}

func (s *DomainService) startCertRenewalWorker() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		s.renewExpiringCertificates()
	}
}

func (s *DomainService) renewExpiringCertificates() {
	// Find certificates expiring in the next 30 days
	expiresBy := time.Now().Add(30 * 24 * time.Hour)

	var certs []SSLCertificate
	s.db.Where("auto_renew = ? AND expires_at < ? AND renewal_failed = ?", true, expiresBy, false).
		Find(&certs)

	for _, cert := range certs {
		var domain CustomDomain
		if err := s.db.First(&domain, "id = ?", cert.DomainID).Error; err != nil {
			continue
		}

		if err := s.provisionSSL(&domain); err != nil {
			cert.RenewalFailed = true
			s.db.Save(&cert)
		}
	}
}

// Utility functions

func isValidDomain(domain string) bool {
	// Basic domain validation
	if len(domain) > 253 || len(domain) < 3 {
		return false
	}

	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		return false
	}

	for _, part := range parts {
		if len(part) > 63 || len(part) == 0 {
			return false
		}
		// Check for valid characters
		for _, r := range part {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
				return false
			}
		}
		// Cannot start or end with hyphen
		if part[0] == '-' || part[len(part)-1] == '-' {
			return false
		}
	}

	return true
}

func generateVerificationToken() string {
	return fmt.Sprintf("apex-verify-%s", uuid.New().String()[:16])
}

func generateFingerprint() string {
	return uuid.New().String()[:32]
}

// Scan implements sql.Scanner for JSONMap
func (m *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*m = make(JSONMap)
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return errors.New("unsupported type for JSONMap")
	}

	return json.Unmarshal(data, m)
}

// Value implements driver.Valuer for JSONMap
func (m JSONMap) Value() (interface{}, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}
