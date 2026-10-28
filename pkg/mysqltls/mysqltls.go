// Package mysqltls configures certificate-verified TLS for MySQL connections.
package mysqltls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/go-sql-driver/mysql"
)

const configName = "trackline"

// RegisterFromEnv registers the Trackline MySQL TLS configuration when
// MYSQL_TLS_CA_FILE points to a PEM-encoded certificate authority bundle.
// A missing path leaves local/default MySQL TLS configuration unchanged.
func RegisterFromEnv() error {
	caPath := os.Getenv("MYSQL_TLS_CA_FILE")
	if caPath == "" {
		return nil
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return fmt.Errorf("read MySQL CA certificate: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return fmt.Errorf("parse MySQL CA certificate: no certificates found")
	}
	if err := mysql.RegisterTLSConfig(configName, &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    rootCAs,
	}); err != nil {
		return fmt.Errorf("register MySQL TLS config: %w", err)
	}
	return nil
}
