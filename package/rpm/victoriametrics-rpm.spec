Name:    victoriametrics
Version: 1.23.0
Release: 3
Summary: The best long-term remote storage for Prometheus

Group:   Development Tools
License: ASL 2.0
URL: https://github.com/VictoriaMetrics/VictoriaMetrics/releases/download/v%{version}/victoria-metrics-v%{version}.tar.gz
Source0: %{name}.service

# Use systemd for fedora >= 18, rhel >=7, SUSE >= 12 SP1 and openSUSE >= 42.1
%define use_systemd (0%{?fedora} && 0%{?fedora} >= 18) || (0%{?rhel} && 0%{?rhel} >= 7) || (!0%{?is_opensuse} && 0%{?suse_version} >=1210) || (0%{?is_opensuse} && 0%{?sle_version} >= 120100)

%description
VictoriaMetrics - the best long-term remote storage for Prometheus

%prep
curl -L %{url} > victoria-metrics.tar.gz
tar -zxf victoria-metrics.tar.gz

%install
%{__install} -m 0755 -d %{buildroot}%{_bindir}
cp victoria-metrics-prod %{buildroot}%{_bindir}/victoria-metrics-prod
%{__install} -m 0755 -d %{buildroot}/var/lib/victoria-metrics-data

%if %{use_systemd}
%{__mkdir} -p %{buildroot}%{_unitdir}
%{__install} -m644 %{SOURCE0} \
    %{buildroot}%{_unitdir}/%{name}.service
%endif

%post
%if %use_systemd
/usr/bin/systemctl daemon-reload
%endif

%preun
%if %use_systemd
/usr/bin/systemctl stop %{name}
%endif

%postun
%if %use_systemd
/usr/bin/systemctl daemon-reload
%endif

%files
%{_bindir}/victoria-metrics-prod
/var/lib/victoria-metrics-data
%if %{use_systemd}
%{_unitdir}/%{name}.service
%endif

