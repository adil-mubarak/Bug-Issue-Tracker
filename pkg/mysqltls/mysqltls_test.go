package mysqltls

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRegisterFromEnv(t *testing.T) {
	t.Run("no CA path leaves default config alone", func(t *testing.T) {
		t.Setenv("MYSQL_TLS_CA_FILE", "")
		if err := RegisterFromEnv(); err != nil {
			t.Fatalf("RegisterFromEnv() error = %v", err)
		}
	})

	t.Run("invalid PEM rejected", func(t *testing.T) {
		caPath := filepath.Join(t.TempDir(), "ca.pem")
		if err := os.WriteFile(caPath, []byte("not a certificate"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MYSQL_TLS_CA_FILE", caPath)
		if err := RegisterFromEnv(); err == nil {
			t.Fatal("RegisterFromEnv() accepted invalid PEM")
		}
	})

	t.Run("valid CA registered", func(t *testing.T) {
		caPath := filepath.Join(t.TempDir(), "ca.pem")
		caPEM := testCAPEM(t)
		if err := os.WriteFile(caPath, caPEM, 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("MYSQL_TLS_CA_FILE", caPath)
		if err := RegisterFromEnv(); err != nil {
			t.Fatalf("RegisterFromEnv() error = %v", err)
		}
	})
}

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
