package auth

import (
	"github.com/golgoth31/sreportal/internal/config"
	domainportal "github.com/golgoth31/sreportal/internal/domain/portal"
)

func jwtAuthViewToConfig(v *domainportal.PortalJWTAuthView) config.JWTAuthConfig {
	if v == nil {
		return config.JWTAuthConfig{}
	}
	issuers := make([]config.JWTIssuerConfig, len(v.Issuers))
	for i := range v.Issuers {
		iss := v.Issuers[i]
		rc := iss.RequiredClaims
		if rc == nil {
			rc = map[string]string{}
		}
		issuers[i] = config.JWTIssuerConfig{
			Name:           iss.Name,
			IssuerURL:      iss.IssuerURL,
			Audience:       iss.Audience,
			JWKSURL:        iss.JWKSURL,
			RequiredClaims: rc,
		}
	}
	return config.JWTAuthConfig{
		Enabled: v.Enabled,
		Issuers: issuers,
	}
}
