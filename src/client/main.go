package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/spiffe/go-spiffe/v2/logger"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const (
	productAPIURL = "https://host.docker.internal:9443/"
	port          = 8080
)

var (
	x509Source         = &workloadapi.X509Source{}
	bundleSource       = &workloadapi.BundleSource{}
	socketPathFlag     = flag.String("agentSocketPath", "/run/spire/sockets/agent.sock", "Agent named pipe name")
	customerAPIURLFlag = flag.String("customerAPIURL", "https://api.api-ns.svc.cluster.local:9001", "Agent named pipe name")
	log                = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
)

type CustomersResponse struct {
	Customers []*Customer `json:"customers"`
}

type Customer struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type ProductsResponse struct {
	Products []*Product `json:"products"`
}

type Product struct {
	Name  string `json:"name"`
	Stock int    `json:"stock"`
}

func getCustomers(jwtSVID *jwtsvid.SVID) ([]*Customer, error) {
	tlsConfig := tlsconfig.MTLSClientConfig(x509Source, bundleSource, tlsconfig.AuthorizeAny())
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	customerAPIURL := *customerAPIURLFlag

	req, err := http.NewRequest("GET", customerAPIURL+"/customers", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtSVID.Marshal()))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to %q: %v", customerAPIURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	listResp := new(CustomersResponse)
	if err := json.NewDecoder(resp.Body).Decode(listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp.Customers, nil
}

func getProducts(jwtSVID *jwtsvid.SVID) ([]*Product, error) {
	serverID := spiffeid.RequireFromString("spiffe://example.org/products-api")
	tlsConfig := tlsconfig.MTLSClientConfig(x509Source, bundleSource, tlsconfig.AuthorizeID(serverID))
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}

	req, err := http.NewRequest("GET", productAPIURL+"/products", nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", jwtSVID.Marshal()))

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error connecting to %q: %v", productAPIURL, err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	listResp := new(ProductsResponse)
	if err := json.NewDecoder(resp.Body).Decode(listResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return listResp.Products, nil
}

type handler struct {
	jwtSource *workloadapi.JWTSource
}

func (h *handler) indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Fetch JWT SVID and add it to `Authorization` header,
	// It is possible to fetch JWT SVID using `workloadapi.FetchJWTSVID`
	svid, err := h.jwtSource.FetchJWTSVID(context.Background(), jwtsvid.Params{
		Audience: "aud",
	})
	if err != nil {
		log.Error("Failed to fetch JWT SVID", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	}

	log.Info("JWT SVID fetched", "token", svid.Marshal())

	customers, customersErr := getCustomers(svid)
	products, productsErr := getProducts(svid)
	if customersErr != nil {
		log.Error("Failed to get customers", "error", customersErr)
	}
	if productsErr != nil {
		log.Error("Failed to get products", "error", productsErr)
	}

	page.Execute(w, map[string]interface{}{
		"Customers":    customers,
		"CustomersErr": customersErr,
		"Products":     products,
		"ProductsErr":  productsErr,
		"LastUpdated":  time.Now(),
	})
}

func healthy(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func subjectKeyIDToString(ski []byte) string {
	serialHex := fmt.Sprintf("%x", ski)
	if len(serialHex)%2 == 1 {
		serialHex = "0" + serialHex
	}

	return serialHex
}

func main() {
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	socketPath := "unix://" + *socketPathFlag
	clientOptions := workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath),
		workloadapi.WithLogger(logger.Std))
	var err error
	x509Source, err = workloadapi.NewX509Source(ctx, clientOptions)
	if err != nil {
		log.Error("Failed to create X509Source", "error", err)
		os.Exit(1)
	}
	defer x509Source.Close()
	svid, err := x509Source.GetX509SVID()
	if err == nil {
		log.Info("SVID found", "spiffe_id", svid.ID.String())
	}

	bundleSource, err = workloadapi.NewBundleSource(ctx, clientOptions)
	if err != nil {
		log.Error("Failed to create BundleSource", "error", err)
		os.Exit(1)
	}
	defer bundleSource.Close()

	// Create a JWTSource to fetch SVIDs
	jwtSource, err := workloadapi.NewJWTSource(ctx, clientOptions)
	if err != nil {
		log.Error("Failed to create JWTSource", "error", err)
		os.Exit(1)
	}
	defer jwtSource.Close()

	h := &handler{
		jwtSource: jwtSource,
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-x509Source.Updated():
				x509SVID, err := x509Source.GetX509SVID()
				if err != nil {
					log.Error("Failed to get X509SVID", "error", err)
					continue
				}
				log.Info("SVID found", "spiffe_id", x509SVID.ID.String(),
					"subject_key_id", subjectKeyIDToString(x509SVID.Certificates[0].SubjectKeyId),
					"authority_key_id", subjectKeyIDToString(x509SVID.Certificates[0].AuthorityKeyId))
			}
		}
	}()

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}
	http.HandleFunc("/", h.indexHandler)
	http.HandleFunc("/healthy", healthy)

	log.Info("Webapp listening on port", "port", port)

	err = server.ListenAndServe()
	if err != nil {
		log.Error("Failed to start server", "error", err)
		os.Executable()
	}
}
