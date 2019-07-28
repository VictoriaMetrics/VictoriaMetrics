#!/bin/bash

if ! which rpmbuild 2> /dev/null
then
	echo "*** Fatal: please install rpmbuild"
    exit 1
fi

ARCH="amd64"
if [[ $# -ge 1 ]]
then
    ARCH="$1"
fi

# Map to Debian architecture
if [[ "$ARCH" == "amd64" ]]; then
    RPM_ARCH=x86_64
	EXENAME_SRC="victoria-metrics-prod"
elif [[ "$ARCH" == "arm64" ]]; then
    RPM_ARCH=aarch64
    EXENAME_SRC="victoria-metrics-arm64-prod"
else
    echo "*** Unknown arch $ARCH"
    exit 1
fi

PACKDIR="./package"
TEMPDIR="${PACKDIR}/temp-rpm-${RPM_ARCH}"
EXENAME_DST="victoria-metrics-prod"

# Pull in version info

VERSION=`cat ${PACKDIR}/VAR_VERSION | perl -ne 'chomp and print'`
BUILD=`cat ${PACKDIR}/VAR_BUILD | perl -ne 'chomp and print'`

# Create directories

[[ -d "${TEMPDIR}" ]] && rm -rf "${TEMPDIR}"

mkdir -p "${TEMPDIR}" && echo "*** Created   : ${TEMPDIR}"

echo "*** Version   : ${VERSION}-${BUILD}"
echo "*** Arch      : ${RPM_ARCH}"

OUT_RPM="victoria-metrics-${VERSION}-${BUILD}.${RPM_ARCH}.rpm"

echo "*** Out .rpm  : ${OUT_RPM}"

cat > "${TEMPDIR}/victoria-metrics.spec" <<EOF
Summary: The best long-term remote storage for Prometheus
Name: victoria-metrics
Version: ${VERSION}
Release: ${BUILD}
License: Apache License 2.0
URL: https://victoriametrics.com/
Group: System
Packager: Aliaksandr Valialkin
Requires: libpthread
Requires: libc

%description
VictoriaMetrics is fast, cost-effective and scalable time series database. It can be used as a long-term remote storage for Prometheus.

%files
%attr(0744, root, root) /usr/bin/*
%attr(0644, root, root) /lib/systemd/system/*

%prep
mkdir -p \$RPM_BUILD_ROOT/usr/bin/
mkdir -p \$RPM_BUILD_ROOT/lib/systemd/system/

cp ${PWD}/bin/${EXENAME_SRC} \$RPM_BUILD_ROOT/usr/bin/${EXENAME_DST}
cp ${PWD}/package/victoria-metrics.service \$RPM_BUILD_ROOT/lib/systemd/system/

%post
/usr/bin/systemctl daemon-reload

%preun
/usr/bin/systemctl stop victoria-metrics

%postun
/usr/bin/systemctl daemon-reload
EOF

rpmbuild -bb --target "${RPM_ARCH}" \
	"${TEMPDIR}/victoria-metrics.spec"

cp "${HOME}/rpmbuild/RPMS/${RPM_ARCH}/${OUT_RPM}" "${PACKDIR}"

echo "*** Created   : ${PACKDIR}/${OUT_RPM}"
