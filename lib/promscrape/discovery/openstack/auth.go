package openstack

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"time"
)

// authResponse - identity api response
// https://docs.openstack.org/api-ref/identity/v3/?expanded=create-credential-detail,password-authentication-with-unscoped-authorization-detail#authentication-and-token-management
type authResponse struct {
	Token struct {
		ExpiresAt time.Time     `json:"expires_at,omitempty"`
		Catalog   []catalogItem `json:"catalog,omitempty"`
	}
}

type catalogItem struct {
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Endpoints []endpoint `json:"endpoints"`
}

// openstack api endpoint
// https://docs.openstack.org/api-ref/identity/v3/?expanded=create-credential-detail,password-authentication-with-unscoped-authorization-detail,token-authentication-with-scoped-authorization-detail#list-endpoints
type endpoint struct {
	RegionID   string `json:"region_id"`
	RegionName string `json:"region_name"`
	URL        string `json:"url"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Interface  string `json:"interface"`
}

// getComputeEndpointURL extracts compute url endpoint with given filters from keystone catalog
func getComputeEndpointURL(catalog []catalogItem, availability, region string) (*url.URL, error) {
	for _, eps := range catalog {
		if eps.Type == "compute" {
			for _, ep := range eps.Endpoints {
				if ep.Interface == availability && (len(region) == 0 || region == ep.RegionID || region == ep.RegionName) {
					return url.Parse(ep.URL)
				}
			}
		}
	}
	return nil, fmt.Errorf("cannot excract compute url from catalog, availability: %s, region: %s ", availability, region)
}

// buildAuthRequestBody - builds request for authentication.
func buildAuthRequestBody(sdc *SDConfig) ([]byte, error) {

	// fast path
	if len(sdc.Password) == 0 && len(sdc.ApplicationCredentialID) == 0 && len(sdc.ApplicationCredentialName) == 0 {
		return nil, fmt.Errorf("password and application credentials is missing")
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
		Identity identityReq            `json:"identity"`
		Scope    map[string]interface{} `json:"scope,omitempty"`
	}

	type request struct {
		Auth authReq `json:"auth"`
	}

	// Populate the request structure based on the provided arguments. Create and return an error
	// if insufficient or incompatible information is present.
	var req request

	if len(sdc.Password) == 0 {
		// There are three kinds of possible application_credential requests
		// 1. application_credential id + secret
		// 2. application_credential name + secret + user_id
		// 3. application_credential name + secret + username + domain_id / domain_name
		if len(sdc.ApplicationCredentialID) > 0 {
			if len(sdc.ApplicationCredentialSecret) == 0 {
				return nil, fmt.Errorf("ApplicationCredentialSecret is empty")
			}
			req.Auth.Identity.Methods = []string{"application_credential"}
			req.Auth.Identity.ApplicationCredential = &applicationCredentialReq{
				ID:     &sdc.ApplicationCredentialID,
				Secret: &sdc.ApplicationCredentialSecret,
			}

			// fast path unscoped
			return json.Marshal(req)
		}

		// application_credential_name auth
		if len(sdc.ApplicationCredentialSecret) == 0 {
			return nil, fmt.Errorf("application_credential_name is not empty and application_credential_secret is empty")
		}

		var userRequest *userReq

		if len(sdc.UserID) > 0 {
			// UserID could be used without the domain information
			userRequest = &userReq{
				ID: &sdc.UserID,
			}
		}

		if userRequest == nil && len(sdc.Username) == 0 {
			// Make sure that Username or UserID are provided
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

		// Make sure that domain_id or domain_name are provided among username
		if userRequest == nil {
			return nil, fmt.Errorf("domain_id and domain_name is empty for application_credential_name auth")
		}

		req.Auth.Identity.Methods = []string{"application_credential"}
		req.Auth.Identity.ApplicationCredential = &applicationCredentialReq{
			Name:   &sdc.ApplicationCredentialName,
			User:   userRequest,
			Secret: &sdc.ApplicationCredentialSecret,
		}

		// fast path unscoped
		return json.Marshal(req)
	}

	// Password authentication.
	req.Auth.Identity.Methods = append(req.Auth.Identity.Methods, "password")

	// At least one of Username and UserID must be specified.
	if len(sdc.Username) == 0 && len(sdc.UserID) == 0 {
		return nil, fmt.Errorf("username and userid is empty for username/password auth")
	}

	if len(sdc.Username) > 0 {
		// If Username is provided, UserID may not be provided.
		if len(sdc.UserID) > 0 {
			return nil, fmt.Errorf("both username and userid is present")
		}

		// Either DomainID or DomainName must also be specified.
		if len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
			return nil, fmt.Errorf(" domain_id or domain_name is missing for username/password auth: %s", sdc.Username)
		}

		if len(sdc.DomainID) > 0 {
			if sdc.DomainName != "" {
				return nil, fmt.Errorf("both domain_id and domain_name is present")
			}

			// Configure the request for Username and Password authentication with a DomainID.
			if len(sdc.Password) > 0 {
				req.Auth.Identity.Password = &passwordReq{
					User: userReq{
						Name:     &sdc.Username,
						Password: &sdc.Password,
						Domain:   &domainReq{ID: &sdc.DomainID},
					},
				}
			}
		}

		if len(sdc.DomainName) > 0 {
			// Configure the request for Username and Password authentication with a DomainName.
			if len(sdc.Password) > 0 {
				req.Auth.Identity.Password = &passwordReq{
					User: userReq{
						Name:     &sdc.Username,
						Password: &sdc.Password,
						Domain:   &domainReq{Name: &sdc.DomainName},
					},
				}
			}
		}
	}

	if len(sdc.UserID) > 0 {
		// If UserID is specified, neither DomainID nor DomainName may be.
		if len(sdc.DomainID) > 0 {
			return nil, fmt.Errorf("both user_id and domain_id is present")
		}
		if len(sdc.DomainName) > 0 {
			return nil, fmt.Errorf("both user_id and domain_name is present")
		}

		// Configure the request for UserID and Password authentication.
		if len(sdc.Password) > 0 {
			req.Auth.Identity.Password = &passwordReq{
				User: userReq{
					ID:       &sdc.UserID,
					Password: &sdc.Password,
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

// buildScope - adds scope information into auth request
// https://docs.openstack.org/api-ref/identity/v3/?expanded=password-authentication-with-scoped-authorization-detail#password-authentication-with-unscoped-authorization
func buildScope(sdc *SDConfig) (map[string]interface{}, error) {

	// fast path
	if len(sdc.ProjectName) == 0 && len(sdc.ProjectID) == 0 && len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
		return nil, nil
	}

	if len(sdc.ProjectName) > 0 {
		// ProjectName provided: either DomainID or DomainName must also be supplied.
		// ProjectID may not be supplied.
		if len(sdc.DomainID) == 0 && len(sdc.DomainName) == 0 {
			return nil, fmt.Errorf("both domain_id and domain_name present")
		}
		if len(sdc.ProjectID) > 0 {
			return nil, fmt.Errorf("both domain_id and domain_name present")
		}

		if len(sdc.DomainID) > 0 {

			// ProjectName + DomainID
			return map[string]interface{}{
				"project": map[string]interface{}{
					"name":   &sdc.ProjectName,
					"domain": map[string]interface{}{"id": &sdc.DomainID},
				},
			}, nil
		}

		if len(sdc.DomainName) > 0 {

			// ProjectName + DomainName
			return map[string]interface{}{
				"project": map[string]interface{}{
					"name":   &sdc.ProjectName,
					"domain": map[string]interface{}{"name": &sdc.DomainName},
				},
			}, nil
		}
	} else if len(sdc.ProjectID) > 0 {
		// ProjectID provided. ProjectName, DomainID, and DomainName may not be provided.
		if len(sdc.DomainID) > 0 {
			return nil, fmt.Errorf("both project_id and domain_id present")
		}
		if len(sdc.DomainName) > 0 {
			return nil, fmt.Errorf("both project_id and domain_name present")
		}

		// ProjectID
		return map[string]interface{}{
			"project": map[string]interface{}{
				"id": &sdc.ProjectID,
			},
		}, nil
	} else if len(sdc.DomainID) > 0 {
		// DomainID provided. ProjectID, ProjectName, and DomainName may not be provided.
		if len(sdc.DomainName) > 0 {
			return nil, fmt.Errorf("both domain_id and domain_name present")
		}

		// DomainID
		return map[string]interface{}{
			"domain": map[string]interface{}{
				"id": &sdc.DomainID,
			},
		}, nil
	} else if len(sdc.DomainName) > 0 {

		// DomainName
		return map[string]interface{}{
			"domain": map[string]interface{}{
				"name": &sdc.DomainName,
			},
		}, nil
	}

	return nil, nil
}

// readCredentialsFromEnv - obtains serviceDiscoveryConfig from env variables for openstack
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
		Password:                    password,
		ProjectName:                 tenantName,
		ProjectID:                   tenantID,
		DomainName:                  domainName,
		DomainID:                    domainID,
		ApplicationCredentialName:   applicationCredentialName,
		ApplicationCredentialID:     applicationCredentialID,
		ApplicationCredentialSecret: applicationCredentialSecret,
	}
}
