package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func storeSVIDUpdate(source *workloadapi.X509Source, log *slog.Logger) error {
	x509SVID, err := source.GetX509SVID()
	if err != nil {
		return fmt.Errorf("failed to get SVID: %w", err)
	}
	x509Bundle, err := source.GetX509BundleForTrustDomain(x509SVID.ID.TrustDomain())
	if err != nil {
		return fmt.Errorf("failed to get bundle for trust domain: %w", err)
	}

	// TODO: update to display signing authorities
	log.Info("SVID recieved", "spiffe_id", x509SVID.ID.String(),
		"subject_key_id", subjectKeyIDToString(x509SVID.Certificates[0].SubjectKeyId),
		"authority_key_id", subjectKeyIDToString(x509SVID.Certificates[0].AuthorityKeyId))
	cert, key, err := x509SVID.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marhal SVID: %w", err)
	}

	if err := writeCertificates("svid.pem", cert); err != nil {
		return fmt.Errorf("failed to write certificates on disk; %w", err)
	}

	if err := writeKey("svid.key", key); err != nil {
		return fmt.Errorf("failed to write key on disk; %w", err)
	}

	bundlePem, err := x509Bundle.Marshal()
	if err != nil {
		return fmt.Errorf("failed to get marshal bundle: %w", err)
	}

	if err := writeCertificates("bundle.pem", bundlePem); err != nil {
		return fmt.Errorf("failed to write bundles on disk; %w", err)
	}

	return nil
}

func writeCertificates(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0644) // nolint: gosec // expected permission for certificates
}

func writeKey(filename string, data []byte) error {
	return os.WriteFile(filename, data, 0600)
}

func subjectKeyIDToString(ski []byte) string {
	serialHex := fmt.Sprintf("%x", ski)
	if len(serialHex)%2 == 1 {
		serialHex = "0" + serialHex
	}

	return serialHex
}
