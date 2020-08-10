#!/bin/bash

set -e

usage() {
    cat << EOF
USAGE: $0 [-v VAR_VERSION] [-b VAR_BUILD] [-e EXENAME_BASE] [ARCH]

Build debian package of various VictoriaMetrics binaries. Required binary must
exist in bin/. To build the binary run:
    make EXENAME_BASE-ARCH-prod

    -v VAR_VERSION:  debian package version string [default $(dirname "$0")/VAR_VERSION contents]
    -b VAR_BUILD:    debian package build string [default $(dirname "$0")/VAR_BUILD contents]
    -e EXENAME_BASE: supported: victoria-metrics, vmagent, vmalert [default victoria-metrics]

    ARCH:            amd64, arm or arm64 [default amd64]

Basic systemd units are included in the package. In most cases you probably
should change the unit before build in $(dirname "$0")/EXENAME_BASE.service or
create an override systemd unit after install.
EOF
    exit 1
}

PACKDIR=$(dirname "$0")
ARCH="amd64"
EXENAME_BASE="victoria-metrics"
VERSION=`cat ${PACKDIR}/VAR_VERSION | perl -ne 'chomp and print'`
BUILD=`cat ${PACKDIR}/VAR_BUILD | perl -ne 'chomp and print'`

while getopts 'v:b:e:h' c; do
    case "$c" in
        b) BUILD="$OPTARG";;
        v) VERSION="$OPTARG";;
        e) EXENAME_BASE="$OPTARG";;
        h) usage;;
        *) usage;;
    esac
done
shift $(($OPTIND - 1))
if [ $# -ge 1 ]; then
    ARCH="$1"
fi

# Map to Debian architecture
if [[ "$ARCH" == "amd64" ]]; then
    DEB_ARCH=amd64
    EXENAME_SRC="${EXENAME_BASE}-prod"
elif [[ "$ARCH" == "arm64" ]]; then
    DEB_ARCH=arm64
    EXENAME_SRC="${EXENAME_BASE}-arm64-prod"
elif [[ "$ARCH" == "arm" ]]; then
    DEB_ARCH=armhf
    EXENAME_SRC="${EXENAME_BASE}-arm-prod"
else
    echo "*** Unknown arch $ARCH"
    exit 1
fi

TEMPDIR="${PACKDIR}/temp-deb-${DEB_ARCH}"
EXENAME_DST="${EXENAME_BASE}-prod"

# Create directories

[[ -d "${TEMPDIR}" ]] && rm -rf "${TEMPDIR}"

mkdir -p "${TEMPDIR}" && echo "*** Created   : ${TEMPDIR}"

mkdir -p "${TEMPDIR}/usr/bin/"
mkdir -p "${TEMPDIR}/lib/systemd/system/"

echo "*** Version   : ${VERSION}-${BUILD}"
echo "*** Arch      : ${DEB_ARCH}"

OUT_DEB="${EXENAME_BASE}_${VERSION}-${BUILD}_$DEB_ARCH.deb"

echo "*** Out .deb  : ${OUT_DEB}"

# Copy the binary

cp "./bin/${EXENAME_SRC}" "${TEMPDIR}/usr/bin/${EXENAME_DST}"

# Copy supporting files

cp "${PACKDIR}/${EXENAME_BASE}.service" "${TEMPDIR}/lib/systemd/system/"

# Generate debian-binary

echo "2.0" > "${TEMPDIR}/debian-binary"

# Generate control

echo "Version: $VERSION-$BUILD" > "${TEMPDIR}/control"
echo "Installed-Size:" `du -sb "${TEMPDIR}" | awk '{print int($1/1024)}'` >> "${TEMPDIR}/control"
echo "Architecture: $DEB_ARCH" >> "${TEMPDIR}/control"
cat "${PACKDIR}/deb/${EXENAME_BASE}.control" >> "${TEMPDIR}/control"

# Copy conffile

cp "${PACKDIR}/deb/conffile" "${TEMPDIR}/conffile"

# Copy postinst and postrm

cp "${PACKDIR}/deb/postinst" "${TEMPDIR}/postinst"
cp "${PACKDIR}/deb/${EXENAME_BASE}.prerm" "${TEMPDIR}/prerm"
cp "${PACKDIR}/deb/postrm" "${TEMPDIR}/postrm"

(
    # Generate md5 sums

    cd "${TEMPDIR}"

    find ./usr ./lib -type f | while read i ; do
        md5sum "$i" | sed 's/\.\///g' >> md5sums
    done

    # Archive control

    chmod 644 control md5sums
    chmod 755 postinst postrm prerm
    fakeroot -- tar -c --xz -f ./control.tar.xz ./control ./md5sums ./postinst ./prerm ./postrm

    # Archive data

    fakeroot -- tar -c --xz -f ./data.tar.xz ./usr ./lib

    # Make final archive

    fakeroot -- ar -cr "${OUT_DEB}" debian-binary control.tar.xz data.tar.xz
)

ls -lh "${TEMPDIR}/${OUT_DEB}"

cp "${TEMPDIR}/${OUT_DEB}" "${PACKDIR}"

echo "*** Created   : ${PACKDIR}/${OUT_DEB}"
