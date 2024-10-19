package main

import (
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
	log            = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
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

func start() error {
	flag.Parse()

	log.Info("Reading configuration file", "path", *configFilePath)
	var c config
	if err := hclsimple.DecodeFile(*configFilePath, nil, &c); err != nil {
		return fmt.Errorf("error parsing configuration file: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clientOptions := workloadapi.WithClientOptions(workloadapi.WithAddr(c.AgentSock), workloadapi.WithLogger(logger.Std))

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

	go monitorSVIDUpdates(ctx, source)

	jwtSource, err := workloadapi.NewJWTSource(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("unable to create JWTSource: %w", err)
	}
	defer jwtSource.Close()

	go monitorJWTUpdates(ctx, jwtSource)

	auth := &authenticator{
		jwtSource: jwtSource,
		audiences: []string{"aud"},
		log:       log,
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslcert=%s sslkey=%s sslrootcert=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBName, "svid.pem", "svid.key", "bundle.pem")
	h := NewHandler(connStr, c.AgentSock, log)
	http.Handle("/customers", auth.authenticateClient(http.HandlerFunc(h.CustomersList)))
	http.Handle("/customer/insert", auth.authenticateClient(http.HandlerFunc(h.CustomerInsert)))

	td := spiffeid.RequireTrustDomainFromString("cluster.demo")
	tlsConfig := tlsconfig.MTLSServerConfig(source, bundleSource, tlsconfig.AuthorizeMemberOf(td))
	server := &http.Server{
		Addr:              ":" + strconv.Itoa(c.Port),
		TLSConfig:         tlsConfig,
		ReadHeaderTimeout: time.Second * 10,
	}

	log.Info("Service starting", "host", c.Host, "port", c.Port)
	return server.ListenAndServeTLS("", "")
}

func monitorSVIDUpdates(ctx context.Context, source *workloadapi.X509Source) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-source.Updated():
			if err := storeSVIDUpdate(source, log); err != nil {
				log.Error("Failed to store SVID update", "error", err)
			}
		}
	}
}

func monitorJWTUpdates(ctx context.Context, jwtSource *workloadapi.JWTSource) {
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

			jwtSVID, err := jwtSource.FetchJWTSVID(ctx, jwtsvid.Params{Audience: "aud"})
			if err != nil {
				log.Error("Failed to fetch JWT SVID", "error", err)
				continue
			}
			log.Info("JWT SVID fetched", "marshal", jwtSVID.Marshal())
		}
	}
}

func main() {
	if err := start(); err != nil {
		log.Error("Service failed to start", "error", err)
		os.Exit(1)
	}
}
