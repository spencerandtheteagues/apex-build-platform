package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Plan     string `json:"plan"` // free, pro, enterprise
	jwt.RegisteredClaims
}

type JWTService struct {
	secretKey     []byte
	refreshSecret []byte
	issuer        string
}

func NewJWTService(secretKey, refreshSecret, issuer string) *JWTService {
	return &JWTService{
		secretKey:     []byte(secretKey),
		refreshSecret: []byte(refreshSecret),
		issuer:        issuer,
	}
}

// GenerateTokens creates both access and refresh tokens
func (j *JWTService) GenerateTokens(userID uint, username, email, role, plan string) (accessToken, refreshToken string, err error) {
	// Access Token (short lived - 15 minutes)
	accessClaims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		Plan:     plan,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   username,
		},
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString(j.secretKey)
	if err != nil {
		return "", "", err
	}

	// Refresh Token (long lived - 7 days)
	refreshClaims := Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		Plan:     plan,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    j.issuer,
			Subject:   username,
		},
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString(j.refreshSecret)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateAccessToken validates and parses access token
func (j *JWTService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

// ValidateRefreshToken validates and parses refresh token
func (j *JWTService) ValidateRefreshToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("invalid signing method")
		}
		return j.refreshSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid refresh token claims")
}

// RefreshAccessToken generates new access token from valid refresh token
func (j *JWTService) RefreshAccessToken(refreshToken string) (string, error) {
	claims, err := j.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", err
	}

	// Generate new access token with same user info
	accessToken, _, err := j.GenerateTokens(
		claims.UserID,
		claims.Username,
		claims.Email,
		claims.Role,
		claims.Plan,
	)

	return accessToken, err
}