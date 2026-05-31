package storage

import (
	"fmt"
	"os"
)

// NewFromEnv returns R2Provider if R2_ACCOUNT_ID+R2_ACCESS_KEY_ID+R2_SECRET_ACCESS_KEY+R2_BUCKET_NAME are all set,
// otherwise returns LocalProvider with baseDir from UPLOAD_DIR env (default "./uploads").
// Returns an error (instead of panicking) so the caller can emit a clear diagnostic and exit cleanly.
func NewFromEnv() (Provider, error) {
	// Check R2 environment variables
	accountID := os.Getenv("R2_ACCOUNT_ID")
	accessKeyID := os.Getenv("R2_ACCESS_KEY_ID")
	secretAccessKey := os.Getenv("R2_SECRET_ACCESS_KEY")
	bucketName := os.Getenv("R2_BUCKET_NAME")

	// If all R2 vars are set, try R2
	if accountID != "" && accessKeyID != "" && secretAccessKey != "" && bucketName != "" {
		r2Provider, err := NewR2Provider(accountID, accessKeyID, secretAccessKey, bucketName)
		if err != nil {
			fmt.Printf("Warning: R2 credentials provided but failed to initialize R2 provider (%v), falling back to local storage\n", err)
		} else {
			fmt.Printf("Storage: Using Cloudflare R2 (bucket: %s)\n", bucketName)
			return r2Provider, nil
		}
	}

	// Fall back to local storage
	uploadDir := os.Getenv("UPLOAD_DIR")
	if uploadDir == "" {
		uploadDir = "./uploads"
	}

	localProvider, err := NewLocalProvider(uploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize local storage provider (dir: %s): %w", uploadDir, err)
	}

	fmt.Printf("Storage: Using local disk (dir: %s)\n", uploadDir)
	return localProvider, nil
}