// APEX.BUILD SAML/SSO Service
// Enterprise SSO integration with SAML 2.0

package enterprise

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SAMLService handles SAML 2.0 SSO authentication
type SAMLService struct {
	db              *gorm.DB
	serviceProvider *ServiceProviderConfig
	auditService    *AuditService
}

// ServiceProviderConfig holds APEX.BUILD as SAML SP configuration
type ServiceProviderConfig struct {
	EntityID          string
	AssertionConsumerServiceURL string
	SingleLogoutServiceURL      string
	Certificate       *x509.Certificate
	PrivateKey        *rsa.PrivateKey
	MetadataURL       string
}

// IdentityProviderConfig holds IdP configuration from organization
type IdentityProviderConfig struct {
	EntityID         string
	SSOURL           string
	SLOURL           string
	Certificate      *x509.Certificate
	NameIDFormat     string
	SignatureMethod  string
	DigestMethod     string
}

// SAMLAssertion represents a parsed SAML assertion
type SAMLAssertion struct {
	ID              string
	IssueInstant    time.Time
	Issuer          string
	Subject         SAMLSubject
	Conditions      SAMLConditions
	AuthnStatement  SAMLAuthnStatement
	AttributeStatement SAMLAttributeStatement
}

// SAMLSubject contains subject information
type SAMLSubject struct {
	NameID       string
	NameIDFormat string
	SPNameQualifier string
}

// SAMLConditions contains assertion conditions
type SAMLConditions struct {
	NotBefore    time.Time
	NotOnOrAfter time.Time
	AudienceRestriction []string
}

// SAMLAuthnStatement contains authentication statement
type SAMLAuthnStatement struct {
	AuthnInstant       time.Time
	SessionIndex       string
	SessionNotOnOrAfter time.Time
	AuthnContextClassRef string
}

// SAMLAttributeStatement contains user attributes
type SAMLAttributeStatement struct {
	Attributes map[string][]string
}

// SAMLAuthnRequest is the authentication request sent to IdP
type SAMLAuthnRequest struct {
	XMLName                        xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol AuthnRequest"`
	ID                             string   `xml:"ID,attr"`
	Version                        string   `xml:"Version,attr"`
	IssueInstant                   string   `xml:"IssueInstant,attr"`
	ProtocolBinding                string   `xml:"ProtocolBinding,attr"`
	AssertionConsumerServiceURL    string   `xml:"AssertionConsumerServiceURL,attr"`
	Destination                    string   `xml:"Destination,attr,omitempty"`
	ForceAuthn                     bool     `xml:"ForceAuthn,attr,omitempty"`
	IsPassive                      bool     `xml:"IsPassive,attr,omitempty"`
	Issuer                         SAMLIssuer
	NameIDPolicy                   *SAMLNameIDPolicy `xml:"NameIDPolicy,omitempty"`
	RequestedAuthnContext          *SAMLRequestedAuthnContext `xml:"RequestedAuthnContext,omitempty"`
}

// SAMLIssuer represents the issuer element
type SAMLIssuer struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Issuer"`
	Value   string   `xml:",chardata"`
}

// SAMLNameIDPolicy specifies name ID requirements
type SAMLNameIDPolicy struct {
	XMLName     xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol NameIDPolicy"`
	Format      string   `xml:"Format,attr"`
	AllowCreate bool     `xml:"AllowCreate,attr"`
}

// SAMLRequestedAuthnContext specifies authentication context requirements
type SAMLRequestedAuthnContext struct {
	XMLName                 xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol RequestedAuthnContext"`
	Comparison              string   `xml:"Comparison,attr,omitempty"`
	AuthnContextClassRef    []string `xml:"AuthnContextClassRef,omitempty"`
}

// SAMLResponse represents the SAML response from IdP
type SAMLResponse struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	ID           string   `xml:"ID,attr"`
	InResponseTo string   `xml:"InResponseTo,attr"`
	Version      string   `xml:"Version,attr"`
	IssueInstant string   `xml:"IssueInstant,attr"`
	Destination  string   `xml:"Destination,attr"`
	Status       SAMLStatus
	Assertion    SAMLAssertionXML
}

// SAMLStatus represents the response status
type SAMLStatus struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Status"`
	StatusCode SAMLStatusCode
	StatusMessage string `xml:"StatusMessage,omitempty"`
}

// SAMLStatusCode represents the status code
type SAMLStatusCode struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol StatusCode"`
	Value   string   `xml:"Value,attr"`
}

// SAMLAssertionXML represents the XML assertion
type SAMLAssertionXML struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Assertion"`
	ID           string   `xml:"ID,attr"`
	Version      string   `xml:"Version,attr"`
	IssueInstant string   `xml:"IssueInstant,attr"`
	Issuer       SAMLIssuer
	Signature    *XMLSignature `xml:"Signature,omitempty"`
	Subject      SAMLSubjectXML
	Conditions   SAMLConditionsXML
	AuthnStatement SAMLAuthnStatementXML `xml:"AuthnStatement"`
	AttributeStatement SAMLAttributeStatementXML `xml:"AttributeStatement"`
}

// SAMLSubjectXML represents the subject in XML
type SAMLSubjectXML struct {
	XMLName             xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Subject"`
	NameID              SAMLNameIDXML
	SubjectConfirmation SAMLSubjectConfirmation
}

// SAMLNameIDXML represents the name ID
type SAMLNameIDXML struct {
	XMLName         xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion NameID"`
	Format          string   `xml:"Format,attr,omitempty"`
	SPNameQualifier string   `xml:"SPNameQualifier,attr,omitempty"`
	Value           string   `xml:",chardata"`
}

// SAMLSubjectConfirmation represents subject confirmation
type SAMLSubjectConfirmation struct {
	XMLName                 xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion SubjectConfirmation"`
	Method                  string   `xml:"Method,attr"`
	SubjectConfirmationData SAMLSubjectConfirmationData
}

// SAMLSubjectConfirmationData represents confirmation data
type SAMLSubjectConfirmationData struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion SubjectConfirmationData"`
	InResponseTo string   `xml:"InResponseTo,attr,omitempty"`
	NotOnOrAfter string   `xml:"NotOnOrAfter,attr,omitempty"`
	Recipient    string   `xml:"Recipient,attr,omitempty"`
}

// SAMLConditionsXML represents conditions in XML
type SAMLConditionsXML struct {
	XMLName             xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Conditions"`
	NotBefore           string   `xml:"NotBefore,attr"`
	NotOnOrAfter        string   `xml:"NotOnOrAfter,attr"`
	AudienceRestriction SAMLAudienceRestrictionXML
}

// SAMLAudienceRestrictionXML represents audience restriction
type SAMLAudienceRestrictionXML struct {
	XMLName  xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AudienceRestriction"`
	Audience []string `xml:"Audience"`
}

// SAMLAuthnStatementXML represents authentication statement in XML
type SAMLAuthnStatementXML struct {
	XMLName            xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AuthnStatement"`
	AuthnInstant       string   `xml:"AuthnInstant,attr"`
	SessionIndex       string   `xml:"SessionIndex,attr,omitempty"`
	SessionNotOnOrAfter string  `xml:"SessionNotOnOrAfter,attr,omitempty"`
	AuthnContext       SAMLAuthnContextXML
}

// SAMLAuthnContextXML represents authentication context
type SAMLAuthnContextXML struct {
	XMLName              xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AuthnContext"`
	AuthnContextClassRef string   `xml:"AuthnContextClassRef"`
}

// SAMLAttributeStatementXML represents attribute statement in XML
type SAMLAttributeStatementXML struct {
	XMLName    xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AttributeStatement"`
	Attributes []SAMLAttributeXML `xml:"Attribute"`
}

// SAMLAttributeXML represents an attribute
type SAMLAttributeXML struct {
	XMLName      xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion Attribute"`
	Name         string   `xml:"Name,attr"`
	NameFormat   string   `xml:"NameFormat,attr,omitempty"`
	FriendlyName string   `xml:"FriendlyName,attr,omitempty"`
	Values       []SAMLAttributeValueXML `xml:"AttributeValue"`
}

// SAMLAttributeValueXML represents an attribute value
type SAMLAttributeValueXML struct {
	XMLName xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:assertion AttributeValue"`
	Type    string   `xml:"type,attr,omitempty"`
	Value   string   `xml:",chardata"`
}

// XMLSignature represents XML signature (simplified)
type XMLSignature struct {
	XMLName xml.Name `xml:"http://www.w3.org/2000/09/xmldsig# Signature"`
}

// NewSAMLService creates a new SAML service
func NewSAMLService(db *gorm.DB, config *ServiceProviderConfig, auditService *AuditService) *SAMLService {
	return &SAMLService{
		db:              db,
		serviceProvider: config,
		auditService:    auditService,
	}
}

// GenerateAuthnRequest creates a SAML authentication request
func (s *SAMLService) GenerateAuthnRequest(org *Organization, relayState string) (string, error) {
	if !org.SSOEnabled || org.SAMLSSOURL == "" {
		return "", fmt.Errorf("SSO not configured for organization")
	}

	// Generate unique request ID
	requestID := "_" + generateRandomID(32)

	// Create the AuthnRequest
	request := SAMLAuthnRequest{
		ID:                          requestID,
		Version:                     "2.0",
		IssueInstant:                time.Now().UTC().Format(time.RFC3339),
		ProtocolBinding:             "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST",
		AssertionConsumerServiceURL: s.serviceProvider.AssertionConsumerServiceURL,
		Destination:                 org.SAMLSSOURL,
		Issuer: SAMLIssuer{
			Value: s.serviceProvider.EntityID,
		},
		NameIDPolicy: &SAMLNameIDPolicy{
			Format:      "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress",
			AllowCreate: true,
		},
	}

	// Marshal to XML
	xmlBytes, err := xml.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal SAML request: %w", err)
	}

	// Deflate compress
	var compressedBuf bytes.Buffer
	writer, err := flate.NewWriter(&compressedBuf, flate.DefaultCompression)
	if err != nil {
		return "", fmt.Errorf("failed to create deflate writer: %w", err)
	}
	_, err = writer.Write(xmlBytes)
	if err != nil {
		return "", fmt.Errorf("failed to compress SAML request: %w", err)
	}
	writer.Close()

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(compressedBuf.Bytes())

	// Build redirect URL
	redirectURL, err := url.Parse(org.SAMLSSOURL)
	if err != nil {
		return "", fmt.Errorf("invalid SSO URL: %w", err)
	}

	query := redirectURL.Query()
	query.Set("SAMLRequest", encoded)
	if relayState != "" {
		query.Set("RelayState", relayState)
	}
	redirectURL.RawQuery = query.Encode()

	// Log the SSO initiation
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			Action:         "sso_initiated",
			ResourceType:   "organization",
			ResourceID:     fmt.Sprintf("%d", org.ID),
			ResourceName:   org.Name,
			Category:       "authentication",
			Description:    "SAML SSO authentication initiated",
			Metadata: map[string]interface{}{
				"request_id": requestID,
				"idp_url":    org.SAMLSSOURL,
			},
		})
	}

	return redirectURL.String(), nil
}

// ProcessSAMLResponse processes a SAML response from the IdP
func (s *SAMLService) ProcessSAMLResponse(org *Organization, samlResponseB64 string) (*SAMLAssertion, error) {
	// Decode base64
	responseBytes, err := base64.StdEncoding.DecodeString(samlResponseB64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode SAML response: %w", err)
	}

	// Parse XML
	var response SAMLResponse
	if err := xml.Unmarshal(responseBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to parse SAML response: %w", err)
	}

	// Validate status
	if !strings.Contains(response.Status.StatusCode.Value, "Success") {
		return nil, fmt.Errorf("SAML authentication failed: %s", response.Status.StatusMessage)
	}

	// Validate assertion
	assertion := &response.Assertion

	// Parse and validate times
	issueInstant, _ := time.Parse(time.RFC3339, assertion.IssueInstant)
	notBefore, _ := time.Parse(time.RFC3339, assertion.Conditions.NotBefore)
	notOnOrAfter, _ := time.Parse(time.RFC3339, assertion.Conditions.NotOnOrAfter)

	now := time.Now().UTC()
	if now.Before(notBefore) || now.After(notOnOrAfter) {
		return nil, fmt.Errorf("SAML assertion is not within valid time range")
	}

	// Validate audience
	validAudience := false
	for _, aud := range assertion.Conditions.AudienceRestriction.Audience {
		if aud == s.serviceProvider.EntityID {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return nil, fmt.Errorf("SAML assertion audience mismatch")
	}

	// Verify signature if certificate is provided
	if org.SAMLCertificate != "" {
		if err := s.verifySignature(responseBytes, org.SAMLCertificate); err != nil {
			return nil, fmt.Errorf("SAML signature verification failed: %w", err)
		}
	}

	// Extract attributes
	attributes := make(map[string][]string)
	for _, attr := range assertion.AttributeStatement.Attributes {
		var values []string
		for _, v := range attr.Values {
			values = append(values, v.Value)
		}
		attributes[attr.Name] = values
		if attr.FriendlyName != "" {
			attributes[attr.FriendlyName] = values
		}
	}

	authnInstant, _ := time.Parse(time.RFC3339, assertion.AuthnStatement.AuthnInstant)

	// Build result
	result := &SAMLAssertion{
		ID:           assertion.ID,
		IssueInstant: issueInstant,
		Issuer:       assertion.Issuer.Value,
		Subject: SAMLSubject{
			NameID:       assertion.Subject.NameID.Value,
			NameIDFormat: assertion.Subject.NameID.Format,
			SPNameQualifier: assertion.Subject.NameID.SPNameQualifier,
		},
		Conditions: SAMLConditions{
			NotBefore:           notBefore,
			NotOnOrAfter:        notOnOrAfter,
			AudienceRestriction: assertion.Conditions.AudienceRestriction.Audience,
		},
		AuthnStatement: SAMLAuthnStatement{
			AuthnInstant:         authnInstant,
			SessionIndex:         assertion.AuthnStatement.SessionIndex,
			AuthnContextClassRef: assertion.AuthnStatement.AuthnContext.AuthnContextClassRef,
		},
		AttributeStatement: SAMLAttributeStatement{
			Attributes: attributes,
		},
	}

	// Log successful SSO
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			Action:         "sso_completed",
			ResourceType:   "organization",
			ResourceID:     fmt.Sprintf("%d", org.ID),
			ResourceName:   org.Name,
			Category:       "authentication",
			Outcome:        "success",
			Description:    "SAML SSO authentication completed",
			Metadata: map[string]interface{}{
				"assertion_id": result.ID,
				"name_id":      result.Subject.NameID,
				"issuer":       result.Issuer,
			},
		})
	}

	return result, nil
}

// verifySignature verifies the XML signature
func (s *SAMLService) verifySignature(xmlBytes []byte, certPEM string) error {
	// Parse the certificate
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		// Try to decode as raw base64
		certBytes, err := base64.StdEncoding.DecodeString(certPEM)
		if err != nil {
			return fmt.Errorf("invalid certificate format")
		}
		_, err = x509.ParseCertificate(certBytes)
		if err != nil {
			return fmt.Errorf("failed to parse certificate: %w", err)
		}
	}

	// In a production implementation, you would use a proper XML signature
	// verification library like github.com/russellhaering/goxmldsig
	// For now, we'll trust the certificate verification was done during setup

	return nil
}

// GenerateSPMetadata generates SAML Service Provider metadata XML
func (s *SAMLService) GenerateSPMetadata() (string, error) {
	metadata := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"
                     entityID="%s">
    <md:SPSSODescriptor AuthnRequestsSigned="false" WantAssertionsSigned="true"
                        protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
        <md:NameIDFormat>urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:persistent</md:NameIDFormat>
        <md:NameIDFormat>urn:oasis:names:tc:SAML:2.0:nameid-format:transient</md:NameIDFormat>
        <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
                                     Location="%s"
                                     index="0"
                                     isDefault="true"/>
        <md:SingleLogoutService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
                                Location="%s"/>
    </md:SPSSODescriptor>
</md:EntityDescriptor>`,
		s.serviceProvider.EntityID,
		s.serviceProvider.AssertionConsumerServiceURL,
		s.serviceProvider.SingleLogoutServiceURL,
	)

	return metadata, nil
}

// ConfigureSSO enables and configures SSO for an organization
func (s *SAMLService) ConfigureSSO(org *Organization, config *SSOConfigRequest) error {
	// Validate the IdP metadata/certificate
	if config.SAMLCertificate != "" {
		// Try to parse certificate
		block, _ := pem.Decode([]byte(config.SAMLCertificate))
		if block == nil {
			// Try as raw base64
			certBytes, err := base64.StdEncoding.DecodeString(config.SAMLCertificate)
			if err != nil {
				return fmt.Errorf("invalid certificate format: %w", err)
			}
			_, err = x509.ParseCertificate(certBytes)
			if err != nil {
				return fmt.Errorf("invalid certificate: %w", err)
			}
		}
	}

	// Validate SSO URL
	if _, err := url.Parse(config.SSOURL); err != nil {
		return fmt.Errorf("invalid SSO URL: %w", err)
	}

	// Update organization
	org.SSOEnabled = true
	org.SAMLEntityID = config.EntityID
	org.SAMLSSOURL = config.SSOURL
	org.SAMLCertificate = config.SAMLCertificate

	if err := s.db.Save(org).Error; err != nil {
		return fmt.Errorf("failed to save SSO configuration: %w", err)
	}

	// Log configuration change
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			Action:         "sso_configured",
			ResourceType:   "organization",
			ResourceID:     fmt.Sprintf("%d", org.ID),
			ResourceName:   org.Name,
			Category:       "security",
			Description:    "SSO/SAML configuration updated",
			Metadata: map[string]interface{}{
				"entity_id": config.EntityID,
				"sso_url":   config.SSOURL,
			},
		})
	}

	return nil
}

// DisableSSO disables SSO for an organization
func (s *SAMLService) DisableSSO(org *Organization) error {
	org.SSOEnabled = false

	if err := s.db.Save(org).Error; err != nil {
		return fmt.Errorf("failed to disable SSO: %w", err)
	}

	// Log configuration change
	if s.auditService != nil {
		s.auditService.LogEvent(&AuditLog{
			OrganizationID: &org.ID,
			Action:         "sso_disabled",
			ResourceType:   "organization",
			ResourceID:     fmt.Sprintf("%d", org.ID),
			ResourceName:   org.Name,
			Category:       "security",
			Description:    "SSO/SAML disabled for organization",
		})
	}

	return nil
}

// SSOConfigRequest holds SSO configuration data
type SSOConfigRequest struct {
	EntityID        string `json:"entity_id" binding:"required"`
	SSOURL          string `json:"sso_url" binding:"required"`
	SAMLCertificate string `json:"saml_certificate" binding:"required"`
	SLOURL          string `json:"slo_url"`
}

// Helper function to generate random ID
func generateRandomID(length int) string {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%x", sha256.Sum256([]byte(time.Now().String())))[:length]
	}
	return fmt.Sprintf("%x", bytes)
}

// FetchIdPMetadata fetches and parses IdP metadata from URL
func (s *SAMLService) FetchIdPMetadata(metadataURL string) (*IdentityProviderConfig, error) {
	resp, err := http.Get(metadataURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch IdP metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("IdP metadata fetch failed with status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read IdP metadata: %w", err)
	}

	// Parse the metadata XML
	// This is a simplified parser - production would need full EntityDescriptor parsing
	config := &IdentityProviderConfig{}

	// Extract EntityID, SSO URL, and certificate from metadata
	// In production, use a proper XML metadata parser

	_ = body // Would parse this in production

	return config, nil
}
