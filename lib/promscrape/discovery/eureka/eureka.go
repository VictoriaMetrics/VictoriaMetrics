package eureka

import (
	"encoding/xml"
	"flag"
	"fmt"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.eurekaSDCheckInterval", 30*time.Second, "Interval for checking for changes in eureka. "+
	"This works only if eureka_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#eureka_sd_configs for details")

// SDConfig represents service discovery config for eureka.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#eureka
type SDConfig struct {
	Server            string                     `yaml:"server,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.ec2SDCheckInterval` command-line option.
}

type applications struct {
	Applications []Application `xml:"application"`
}

// Application - eureka application https://github.com/Netflix/eureka/wiki/Eureka-REST-operations/
type Application struct {
	Name      string     `xml:"name"`
	Instances []Instance `xml:"instance"`
}

// Port - eureka instance port.
type Port struct {
	Port    int  `xml:",chardata"`
	Enabled bool `xml:"enabled,attr"`
}

// Instance - eureka instance https://github.com/Netflix/eureka/wiki/Eureka-REST-operations
type Instance struct {
	HostName         string         `xml:"hostName"`
	HomePageURL      string         `xml:"homePageUrl"`
	StatusPageURL    string         `xml:"statusPageUrl"`
	HealthCheckURL   string         `xml:"healthCheckUrl"`
	App              string         `xml:"app"`
	IPAddr           string         `xml:"ipAddr"`
	VipAddress       string         `xml:"vipAddress"`
	SecureVipAddress string         `xml:"secureVipAddress"`
	Status           string         `xml:"status"`
	Port             Port           `xml:"port"`
	SecurePort       Port           `xml:"securePort"`
	DataCenterInfo   DataCenterInfo `xml:"dataCenterInfo"`
	Metadata         MetaData       `xml:"metadata"`
	CountryID        int            `xml:"countryId"`
	InstanceID       string         `xml:"instanceId"`
}

// MetaData - eureka objects metadata.
type MetaData struct {
	Items []Tag `xml:",any"`
}

// Tag - eureka metadata tag - list of k/v values.
type Tag struct {
	XMLName xml.Name
	Content string `xml:",innerxml"`
}

// DataCenterInfo -eureka datacentre metadata
type DataCenterInfo struct {
	Name     string   `xml:"name"`
	Metadata MetaData `xml:"metadata"`
}

// GetLabels returns Eureka labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	data, err := getAPIResponse(cfg, "/apps")
	if err != nil {
		return nil, err
	}
	apps, err := parseAPIResponse(data)
	if err != nil {
		return nil, err
	}
	return addInstanceLabels(apps), nil
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}

func addInstanceLabels(apps *applications) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, app := range apps.Applications {
		for _, instance := range app.Instances {
			instancePort := 80
			if instance.Port.Port != 0 {
				instancePort = instance.Port.Port
			}
			targetAddress := discoveryutil.JoinHostPort(instance.HostName, instancePort)
			m := promutil.NewLabels(24)
			m.Add("__address__", targetAddress)
			m.Add("instance", instance.InstanceID)
			m.Add("__meta_eureka_app_name", app.Name)
			m.Add("__meta_eureka_app_instance_hostname", instance.HostName)
			m.Add("__meta_eureka_app_instance_homepage_url", instance.HomePageURL)
			m.Add("__meta_eureka_app_instance_statuspage_url", instance.StatusPageURL)
			m.Add("__meta_eureka_app_instance_healthcheck_url", instance.HealthCheckURL)
			m.Add("__meta_eureka_app_instance_ip_addr", instance.IPAddr)
			m.Add("__meta_eureka_app_instance_vip_address", instance.VipAddress)
			m.Add("__meta_eureka_app_instance_secure_vip_address", instance.SecureVipAddress)
			m.Add("__meta_eureka_app_instance_status", instance.Status)
			m.Add("__meta_eureka_app_instance_country_id", strconv.Itoa(instance.CountryID))
			m.Add("__meta_eureka_app_instance_id", instance.InstanceID)
			if instance.Port.Port != 0 {
				m.Add("__meta_eureka_app_instance_port", strconv.Itoa(instance.Port.Port))
				m.Add("__meta_eureka_app_instance_port_enabled", strconv.FormatBool(instance.Port.Enabled))
			}
			if instance.SecurePort.Port != 0 {
				m.Add("__meta_eureka_app_instance_secure_port", strconv.Itoa(instance.SecurePort.Port))
				m.Add("__meta_eureka_app_instance_secure_port_enabled", strconv.FormatBool(instance.SecurePort.Enabled))

			}
			if len(instance.DataCenterInfo.Name) > 0 {
				m.Add("__meta_eureka_app_instance_datacenterinfo_name", instance.DataCenterInfo.Name)
				for _, tag := range instance.DataCenterInfo.Metadata.Items {
					m.Add(discoveryutil.SanitizeLabelName("__meta_eureka_app_instance_datacenterinfo_metadata_"+tag.XMLName.Local), tag.Content)
				}
			}
			for _, tag := range instance.Metadata.Items {
				m.Add(discoveryutil.SanitizeLabelName("__meta_eureka_app_instance_metadata_"+tag.XMLName.Local), tag.Content)
			}
			ms = append(ms, m)
		}
	}
	return ms
}
