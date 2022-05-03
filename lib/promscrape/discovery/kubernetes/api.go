package kubernetes

import (
	"encoding/base64"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"gopkg.in/yaml.v2"
	"net"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// apiConfig contains config for API server
type apiConfig struct {
	aw *apiWatcher
}

type Config struct {
	Kind        string      `yaml:"kind,omitempty"`
	APIVersion  string      `yaml:"apiVersion,omitempty"`
	Preferences Preferences `yaml:"preferences"`
	Clusters    []struct {
		Name    string   `yaml:"name"`
		Cluster *Cluster `yaml:"cluster"`
	} `yaml:"clusters"`
	AuthInfos []struct {
		Name     string    `yaml:"name"`
		AuthInfo *AuthInfo `yaml:"user"`
	} `yaml:"users"`
	Contexts []struct {
		Name    string   `yaml:"name"`
		Context *Context `yaml:"context"`
	} `yaml:"contexts"`
	CurrentContext string `yaml:"current-context"`
}

type Preferences struct {
	Colors bool `yaml:"colors,omitempty"`
}

type Cluster struct {
	LocationOfOrigin         string
	Server                   string `yaml:"server"`
	TLSServerName            string `yaml:"tls-server-name,omitempty"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify,omitempty"`
	CertificateAuthority     string `yaml:"certificate-authority,omitempty"`
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	ProxyURL                 string `yaml:"proxy-url,omitempty"`
}

// AuthInfo contains information that describes identity information.  This is use to tell the kubernetes cluster who you are.
type AuthInfo struct {
	LocationOfOrigin      string
	ClientCertificate     string   `yaml:"client-certificate,omitempty"`
	ClientCertificateData string   `yaml:"client-certificate-data,omitempty"`
	ClientKey             string   `yaml:"client-key,omitempty"`
	ClientKeyData         string   `yaml:"client-key-data,omitempty"`
	Token                 string   `yaml:"token,omitempty"`
	TokenFile             string   `yaml:"tokenFile,omitempty"`
	Impersonate           string   `yaml:"act-as,omitempty"`
	ImpersonateUID        string   `yaml:"act-as-uid,omitempty"`
	ImpersonateGroups     []string `yaml:"act-as-groups,omitempty"`
	ImpersonateUserExtra  []string `yaml:"act-as-user-extra,omitempty"`
	Username              string   `yaml:"username,omitempty"`
	Password              string   `yaml:"password,omitempty"`
}

// Context is a tuple of references to a cluster (how do I communicate with a kubernetes cluster), a user (how do I identify myself), and a namespace (what subset of resources do I want to work with)
type Context struct {
	LocationOfOrigin string
	Cluster          string `yaml:"cluster"`
	AuthInfo         string `yaml:"user"`
	Namespace        string `yaml:"namespace,omitempty"`
}

type ApiConfig struct {
	basicAuth *promauth.BasicAuthConfig
	server    string
	token     string
	tokenFile string
	tlsConfig *promauth.TLSConfig
}

func buildConfig(sdc *SDConfig) (*ApiConfig, error) {

	data, err := fs.ReadFileOrHTTP(sdc.KubeConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot read kubeConfig from %q: %w", sdc.KubeConfig, err)
	}
	var config Config
	if err = yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", sdc.KubeConfig, err)
	}

	var authInfos = make(map[string]*AuthInfo)
	for _, obj := range config.AuthInfos {
		authInfos[obj.Name] = obj.AuthInfo
	}
	var clusterInfos = make(map[string]*Cluster)
	for _, obj := range config.Clusters {
		clusterInfos[obj.Name] = obj.Cluster
	}
	var contexts = make(map[string]*Context)
	for _, obj := range config.Contexts {
		contexts[obj.Name] = obj.Context
	}

	contextName := config.CurrentContext
	configContext, exists := contexts[contextName]
	if !exists {
		return nil, fmt.Errorf("context %q does not exist", contextName)
	}

	clusterInfoName := configContext.Cluster
	configClusterInfo, exists := clusterInfos[clusterInfoName]
	if !exists {
		return nil, fmt.Errorf("cluster %q does not exist", clusterInfoName)
	}

	authInfoName := configContext.AuthInfo
	configAuthInfo, exists := authInfos[authInfoName]
	if authInfoName != "" && !exists {
		return nil, fmt.Errorf("auth info %q does not exist", authInfoName)
	}

	var apiConfig ApiConfig

	apiConfig.tlsConfig = &promauth.TLSConfig{
		CAFile:             configClusterInfo.CertificateAuthority,
		ServerName:         configClusterInfo.TLSServerName,
		InsecureSkipVerify: configClusterInfo.InsecureSkipTLSVerify,
	}

	if len(configClusterInfo.CertificateAuthorityData) != 0 {
		apiConfig.tlsConfig.CA, err = base64.StdEncoding.DecodeString(configClusterInfo.CertificateAuthorityData)
		if err != nil {
			return nil, fmt.Errorf("cannot base64-decode configClusterInfo.CertificateAuthorityData %q: %w", configClusterInfo.CertificateAuthorityData, err)
		}
	}

	if configAuthInfo != nil {
		apiConfig.tlsConfig.CertFile = configAuthInfo.ClientCertificate
		apiConfig.tlsConfig.KeyFile = configAuthInfo.ClientKey
		apiConfig.token = configAuthInfo.Token
		apiConfig.tokenFile = configAuthInfo.TokenFile
		if len(configAuthInfo.ClientCertificateData) != 0 {
			apiConfig.tlsConfig.Cert, err = base64.StdEncoding.DecodeString(configAuthInfo.ClientCertificateData)
			if err != nil {
				return nil, fmt.Errorf("cannot base64-decode configAuthInfo.ClientCertificateData %q: %w", configClusterInfo.CertificateAuthorityData, err)
			}
		}
		if len(configAuthInfo.ClientKeyData) != 0 {
			apiConfig.tlsConfig.Key, err = base64.StdEncoding.DecodeString(configAuthInfo.ClientKeyData)
			if err != nil {
				return nil, fmt.Errorf("cannot base64-decode configAuthInfo.ClientKeyData %q: %w", configClusterInfo.CertificateAuthorityData, err)
			}
		}
		if len(configAuthInfo.Username) > 0 || len(configAuthInfo.Password) > 0 {
			apiConfig.basicAuth = &promauth.BasicAuthConfig{
				Username: configAuthInfo.Username,
				Password: promauth.NewSecret(configAuthInfo.Password),
			}
		}
	}

	apiConfig.server = configClusterInfo.Server

	return &apiConfig, nil
}

func newAPIConfig(sdc *SDConfig, baseDir string, swcFunc ScrapeWorkConstructorFunc) (*apiConfig, error) {
	role := sdc.role()
	switch role {
	case "node", "pod", "service", "endpoints", "endpointslice", "ingress":
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `node`, `pod`, `service`, `endpoints`, `endpointslice` or `ingress`", role)
	}
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.APIServer

	if len(sdc.KubeConfig) != 0 {
		config, err := buildConfig(sdc)
		if err != nil {
			return nil, fmt.Errorf("cannot parse kube config: %w", err)
		}
		acNew, err := promauth.NewConfig(".", nil, config.basicAuth, config.token, config.tokenFile, nil, config.tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %w; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
		apiServer = config.server
	}

	if len(apiServer) == 0 {
		// Assume we run at k8s pod.
		// Discover apiServer and auth config according to k8s docs.
		// See https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#service-account-admission-controller
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_HOST env var; it must be defined when running in k8s; " +
				"probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?")
		}
		if len(port) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_PORT env var; it must be defined when running in k8s; "+
				"KUBERNETES_SERVICE_HOST=%q", host)
		}
		apiServer = "https://" + net.JoinHostPort(host, port)
		tlsConfig := promauth.TLSConfig{
			CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		}
		acNew, err := promauth.NewConfig(".", nil, nil, "", "/var/run/secrets/kubernetes.io/serviceaccount/token", nil, &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %w; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
	}
	if !strings.Contains(apiServer, "://") {
		proto := "http"
		if sdc.HTTPClientConfig.TLSConfig != nil {
			proto = "https"
		}
		apiServer = proto + "://" + apiServer
	}
	for strings.HasSuffix(apiServer, "/") {
		apiServer = apiServer[:len(apiServer)-1]
	}
	aw := newAPIWatcher(apiServer, ac, sdc, swcFunc)
	cfg := &apiConfig{
		aw: aw,
	}
	return cfg, nil
}
