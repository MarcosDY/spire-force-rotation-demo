package service

import (
	"api/pkg/handler"
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/spiffe/go-spiffe/v2/logger"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

var (
	configFilePath = flag.String("config", "service.hcl", "Path to configuration file")
)

type config struct {
	Host      string `hcl:"host"`
	Port      int    `hcl:"port"`
	DBHost    string `hcl:"db_host"`
	DBPort    string `hcl:"db_port"`
	DBUser    string `hcl:"db_user"`
	DBName    string `hcl:"db_name"`
	AgentSock string `hcl:"agent_sock"`
}

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

func Start() error {
	flag.Parse()
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	log := slog.New(logHandler)
	slog.SetDefault(log)

	log.Info("Reading configuration file", "path", *configFilePath)
	var c config
	if err := hclsimple.DecodeFile(*configFilePath, nil, &c); err != nil {
		return fmt.Errorf("error parsing configuration file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientOptions := workloadapi.WithClientOptions(workloadapi.WithAddr(c.AgentSock),
		workloadapi.WithLogger(logger.Std))

	log.Info("Creating X509Source")
	source, err := workloadapi.NewX509Source(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create X509Source: %w", err)
	}
	defer source.Close()

	bundleSource, err := workloadapi.NewBundleSource(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create BundleSource %w", err)
	}
	defer bundleSource.Close()

	log.Info("Storing initial SVID")
	if err := storeSVIDUpdate(source, log); err != nil {
		return fmt.Errorf("failed to store SVID update: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-source.Updated():
				if err := storeSVIDUpdate(source, log); err != nil {
					log.Error("Failed to store SVID update", "error", err)
					continue
				}
			}
		}
	}()

	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslcert=%s sslkey=%s sslrootcert=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBName, "svid.pem", "svid.key", "bundle.pem")
	h := handler.NewHandler(connStr, c.AgentSock, log)

	// Create a JWTSource to validate provided tokens from clients
	jwtSource, err := workloadapi.NewJWTSource(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create JWTSource: %w", err)
	}
	defer jwtSource.Close()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-jwtSource.Updated():
				jwtBundle, err := jwtSource.GetJWTBundleForTrustDomain(spiffeid.RequireTrustDomainFromString("cluster.demo"))
				if err != nil {
					log.Error("Failed to get JWT bundle", "error", err)
					continue
				}

				for skid := range jwtBundle.JWTAuthorities() {
					log.Info("JWT authority found", "key_id", skid)
				}
				jwtSVID, err := jwtSource.FetchJWTSVID(ctx, jwtsvid.Params{
					Audience: "aud",
				})
				if err != nil {
					log.Error("Failed to fetch JWT SVID", "error", err)
					continue
				}
				log.Info("JWT SVID fetched", "marshal", jwtSVID.Marshal())

				if err := storeSVIDUpdate(source, log); err != nil {
					log.Error("Failed to store SVID update", "error", err)
					continue
				}
			}
		}
	}()

	// Add a middleware to validate presented JWT token
	auth := &authenticator{
		jwtSource: jwtSource,
		audiences: []string{"aud"},
		log:       log,
	}

	// TODO: refactor handle structure...
	http.Handle("/customers", auth.authenticateClient(http.HandlerFunc(h.CustomersList)))
	http.Handle("/customer/insert", auth.authenticateClient(http.HandlerFunc(h.CustomerInsert)))

	td := spiffeid.RequireTrustDomainFromString("cluster.demo")
	// Create a `tls.Config` to allow mTLS connections, and verify that presented certificate has SPIFFE ID `spiffe://example.org/client`
	tlsConfig := tlsconfig.MTLSServerConfig(source, bundleSource, tlsconfig.AuthorizeMemberOf(td))
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(c.Port),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Second * 10,
	}

	log.Info("Service starting", "host", c.Host, "port", c.Port)
	return server.ListenAndServeTLS("", "")
}
