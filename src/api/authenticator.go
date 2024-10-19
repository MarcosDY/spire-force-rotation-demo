package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

type authenticator struct {
	// JWT Source used to verify token
	jwtSource *workloadapi.JWTSource
	// Expected audiences
	audiences []string
	log       *slog.Logger
}

func (a *authenticator) authenticateClient(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fields := strings.Fields(req.Header.Get("Authorization"))
		if len(fields) != 2 || fields[0] != "Bearer" {
			a.log.Error("Malformed header")
			http.Error(w, "Invalid or unsupported authorization header", http.StatusUnauthorized)
			return
		}

		token := fields[1]

		displayJWT(req.Context(), a.jwtSource, a.log)

		// Parse and validate token against fetched bundle from jwtSource,
		// an alternative is using `workloadapi.ValidateJWTSVID` that will
		// attest against SPIRE on each call and validate token
		svid, err := jwtsvid.ParseAndValidate(token, a.jwtSource, a.audiences)
		if err != nil {
			fmt.Printf("Failed to parse and validate token: %v", err)
			a.log.Error("Invalid token", "error", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		req = req.WithContext(withSVIDClaims(req.Context(), svid.Claims))
		next.ServeHTTP(w, req)
	})
}

type svidClaimsKey struct{}

func withSVIDClaims(ctx context.Context, claims map[string]interface{}) context.Context {
	return context.WithValue(ctx, svidClaimsKey{}, claims)
}

func svidClaims(ctx context.Context) map[string]interface{} {
	claims, _ := ctx.Value(svidClaimsKey{}).(map[string]interface{})
	return claims
}

func displayJWT(ctx context.Context, jwtSource *workloadapi.JWTSource, log *slog.Logger) {
	jwtBundle, err := jwtSource.GetJWTBundleForTrustDomain(spiffeid.RequireTrustDomainFromString("cluster.demo"))
	if err != nil {
		log.Error("Failed to get JWT bundle", "error", err)
		return
	}

	for skid := range jwtBundle.JWTAuthorities() {
		log.Info("JWT authority found", "key_id", skid)
	}

	jwtSVID, err := jwtSource.FetchJWTSVID(ctx, jwtsvid.Params{Audience: "aud"})
	if err != nil {
		log.Error("Failed to fetch JWT SVID", "error", err)
		return
	}
	log.Info("JWT SVID fetched", "marshal", jwtSVID.Marshal())

}
