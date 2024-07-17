package openstack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// authResponse represents identity api response
//
// See https://docs.openstack.org/api-ref/identity/v3/#authentication-and-token-management
type authResponse struct {
	Token authToken
}

type authToken struct {
	ExpiresAt time.Time     `json:"expires_at,omitempty"`
	Catalog   []catalogItem `json:"catalog,omitempty"`
}

type catalogItem struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Endpoints []endpoint `json:"endpoints"`
}

// openstack api endpoint
//
// See https://docs.openstack.org/api-ref/identity/v3/#list-endpoints
type endpoint struct {
	RegionID   string `json:"region_id"`
	RegionName string `json:"region_name"`
	URL        string `json:"url"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Interface  string `json:"interface"`
}

// getComputeEndpointURL extracts compute endpoint url with given filters from keystone catalog
func getComputeEndpointURL(catalog []catalogItem, availability, region string) (*url.URL, error) {
	for _, eps := range catalog {
		if eps.Type != "compute" {
			continue
		}
		for _, ep := range eps.Endpoints {
			if ep.Interface == availability && (len(region) == 0 || region == ep.RegionID || region == ep.RegionName) {
				return url.Parse(ep.URL)
			}
		}
	}
	return nil, fmt.Errorf("cannot find compute url for the given availability: %q, region: %q", availability, region)
}

// buildAuthRequestBody builds request for authentication
func buildAuthRequestBody(sdc *SDConfig) ([]byte, error) {
	if sdc.Password == nil && len(sdc.ApplicationCredentialID) == 0 && len(sdc.ApplicationCredentialName) == 0 {
		return nil, fmt.Errorf("password and application credentials are missing")
	}
	type domainReq struct {
		ID   *string `json:"id,omitempty"`
		Name *string `json:"name,omitempty"`
	}
	type userReq struct {
		ID       *string    `json:"id,omitempty"`
		Name     *string    `json:"name,omitempty"`
		Password *string    `json:"password,omitempty"`
		Passcode *string    `json:"passcode,omitempty"`
		Domain   *domainReq `json:"domain,omitempty"`
	}
	type passwordReq struct {
		User userReq `json:"user"`
	}
	type tokenReq struct {
		ID string `json:"id"`
	}
	type applicationCredentialReq struct {
		ID     *string  `json:"id,omitempty"`
		Name   *string  `json:"name,omitempty"`
		User   *userReq `json:"user,omitempty"`
		Secret *string  `json:"secret,omitempty"`
	}
	type identityReq struct {
		Methods               []string                  `json:"methods"`
		Password              *passwordReq              `json:"password,omitempty"`
		Token                 *tokenReq                 `json:"token,omitempty"`
		ApplicationCredential *applicationCredentialReq `json:"application_credential,omitempty"`
	}
	type authReq struct {
		Identity identityReq    `json:"identity"`
		Scope    map[string]any `json:"scope,omitempty"`
	}
	type request struct {
		Auth authReq `json:"auth"`
	}

	// Populate the request structure based on the provided arguments. Create and return an error
	// if insufficient or incompatible information is present.
	var req request

	if sdc.Password == nil {
		// There are three kinds of possible application_credential requests
		// 1. application_credential id + secret
		// 2. application_credential name + secret + user_id
		// 3. application_credential name + secret + username + domain_id / domain_name
		if len(sdc.ApplicationCredentialID) > 0 {
			if sdc.ApplicationCredentialSecret == nil {
				return nil, fmt.Errorf("ApplicationCredentialSecret is empty")
			}
			req.Auth.Identity.Methods = []string{"application_credential"}
			secret := sdc.ApplicationCredentialSecret.String()
			req.Auth.Identity.ApplicationCredential = &applicationCredentialReq{
				ID:     &sdc.ApplicationCredentialID,
				Secret: &secret,
			}
			return json.Marshal(req)
		}

		if sdc.ApplicationCredentialSecret == nil {
			return nil, fmt.Errorf("missing application_credential_secret when application_credential_name is set")
		}
		var userRequest *userReq
		if len(sdc.UserID) > 0 {
			// UserID could be used without the domain information
			userRequest = &userReq{
				ID: &sdc.UserID,
			}
		}
		if userRequest == nil && len(sdc.Username) == 0 {
			return nil, fmt.Errorf("username and userid is empty")
		}
		if userRequest == nil && len(sdc.DomainID) > 0 {
			userRequest = &userReq{
				Name:   &sdc.Username,
				Domain: &domainReq{ID: &sdc.DomainID},
			}
		}
		if userRequest == nil && len(sdc.DomainName) > 0 {
			userRequest = &userReq{
				Name:   &sdc.Username,
				Domain: &domainReq{Name: &sdc.DomainName},
			}
		}
		if userRequest == nil {
			return nil, fmt.Errorf("domain_id and domain_name cannot be empty for application_credential_name auth")
		}
		req.Auth.Identity.Methods = []string{"application_credential"}
		secret := sdc.ApplicationCredentialSecret.String()
		req.Auth.Identity.ApplicationCredential = &applicationCredentialReq{
			Name:   &sdc.ApplicationCredentialName,
			User:   userRequest,
			Secret: &secret,
		}
		return json.Marshal(req)
	}

	// Password authentication.
	req.Auth.Identity.Methods = append(req.Auth.Identity.Methods, "password")
	if len(sdc.Username) == 0 && len(sdc.UserID) == 0 {
		return nil, fmt.Errorf("username and userid is empty for username/password auth")
	}
	if len(sdc.Username) > 0 {
		if len(sdc.UserID) > 0 {
			return nil, fmt.Errorf("both username and userid is present")
		}
		if len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
			return nil, fmt.Errorf(" domain_id or domain_name is missing for username/password auth: %s", sdc.Username)
		}
		if len(sdc.DomainID) > 0 {
			if sdc.DomainName != "" {
				return nil, fmt.Errorf("both domain_id and domain_name is present")
			}
			// Configure the request for Username and Password authentication with a DomainID.
			if sdc.Password != nil {
				password := sdc.Password.String()
				req.Auth.Identity.Password = &passwordReq{
					User: userReq{
						Name:     &sdc.Username,
						Password: &password,
						Domain:   &domainReq{ID: &sdc.DomainID},
					},
				}
			}
		}
		if len(sdc.DomainName) > 0 {
			// Configure the request for Username and Password authentication with a DomainName.
			if sdc.Password != nil {
				password := sdc.Password.String()
				req.Auth.Identity.Password = &passwordReq{
					User: userReq{
						Name:     &sdc.Username,
						Password: &password,
						Domain:   &domainReq{Name: &sdc.DomainName},
					},
				}
			}
		}
	}
	if len(sdc.UserID) > 0 {
		if len(sdc.DomainID) > 0 {
			return nil, fmt.Errorf("both user_id and domain_id is present")
		}
		if len(sdc.DomainName) > 0 {
			return nil, fmt.Errorf("both user_id and domain_name is present")
		}
		// Configure the request for UserID and Password authentication.
		if sdc.Password != nil {
			password := sdc.Password.String()
			req.Auth.Identity.Password = &passwordReq{
				User: userReq{
					ID:       &sdc.UserID,
					Password: &password,
				},
			}
		}

	}

	// build scope for password auth
	scope, err := buildScope(sdc)
	if err != nil {
		return nil, err
	}
	if len(scope) > 0 {
		req.Auth.Scope = scope
	}
	return json.Marshal(req)
}

// buildScope adds scope information into auth request
//
// See https://docs.openstack.org/api-ref/identity/v3/#password-authentication-with-unscoped-authorization
func buildScope(sdc *SDConfig) (map[string]any, error) {
	if len(sdc.ProjectName) == 0 && len(sdc.ProjectID) == 0 && len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
		return nil, nil
	}
	if len(sdc.ProjectName) > 0 {
		// ProjectName provided: either DomainID or DomainName must also be supplied.
		// ProjectID may not be supplied.
		if len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
			return nil, fmt.Errorf("domain_id or domain_name must present")
		}
		if len(sdc.DomainID) > 0 {
			return map[string]any{
				"project": map[string]any{
					"name":   &sdc.ProjectName,
					"domain": map[string]any{"id": &sdc.DomainID},
				},
			}, nil
		}
		if len(sdc.DomainName) > 0 {
			return map[string]any{
				"project": map[string]any{
					"name":   &sdc.ProjectName,
					"domain": map[string]any{"name": &sdc.DomainName},
				},
			}, nil
		}
	} else if len(sdc.ProjectID) > 0 {
		return map[string]any{
			"project": map[string]any{
				"id": &sdc.ProjectID,
			},
		}, nil
	} else if len(sdc.DomainID) > 0 {
		if len(sdc.DomainName) > 0 {
			return nil, fmt.Errorf("both domain_id and domain_name present")
		}
		return map[string]any{
			"domain": map[string]any{
				"id": &sdc.DomainID,
			},
		}, nil
	} else if len(sdc.DomainName) > 0 {
		return map[string]any{
			"domain": map[string]any{
				"name": &sdc.DomainName,
			},
		}, nil
	}
	return nil, nil
}

// readCredentialsFromEnv obtains serviceDiscoveryConfig from env variables for openstack
func readCredentialsFromEnv() SDConfig {
	authURL := os.Getenv("OS_AUTH_URL")
	username := os.Getenv("OS_USERNAME")
	userID := os.Getenv("OS_USERID")
	password := os.Getenv("OS_PASSWORD")
	tenantID := os.Getenv("OS_TENANT_ID")
	tenantName := os.Getenv("OS_TENANT_NAME")
	domainID := os.Getenv("OS_DOMAIN_ID")
	domainName := os.Getenv("OS_DOMAIN_NAME")
	applicationCredentialID := os.Getenv("OS_APPLICATION_CREDENTIAL_ID")
	applicationCredentialName := os.Getenv("OS_APPLICATION_CREDENTIAL_NAME")
	applicationCredentialSecret := os.Getenv("OS_APPLICATION_CREDENTIAL_SECRET")
	// If OS_PROJECT_ID is set, overwrite tenantID with the value.
	if v := os.Getenv("OS_PROJECT_ID"); v != "" {
		tenantID = v
	}
	// If OS_PROJECT_NAME is set, overwrite tenantName with the value.
	if v := os.Getenv("OS_PROJECT_NAME"); v != "" {
		tenantName = v
	}
	return SDConfig{
		IdentityEndpoint:            authURL,
		Username:                    username,
		UserID:                      userID,
		Password:                    promauth.NewSecret(password),
		ProjectName:                 tenantName,
		ProjectID:                   tenantID,
		DomainName:                  domainName,
		DomainID:                    domainID,
		ApplicationCredentialName:   applicationCredentialName,
		ApplicationCredentialID:     applicationCredentialID,
		ApplicationCredentialSecret: promauth.NewSecret(applicationCredentialSecret),
	}
}
