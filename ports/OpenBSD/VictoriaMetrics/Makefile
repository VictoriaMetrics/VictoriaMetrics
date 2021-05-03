# $OpenBSD: Makefile,v $

COMMENT =		fast, cost-effective and scalable time series database

GH_ACCOUNT =		VictoriaMetrics
GH_PROJECT =		VictoriaMetrics
GH_TAGNAME =		v1.59.0

CATEGORIES =		databases

HOMEPAGE =		https://victoriametrics.com/

MAINTAINER =		VictoriaMetrics <info@victoriametrics.com>

# Apache License 2.0
PERMIT_PACKAGE =	Yes

WANTLIB =		c pthread

USE_GMAKE =		Yes

MODULES =		lang/go
MODGO_GOPATH =		${MODGO_WORKSPACE}
SUBST_VARS =		VARBASE

do-build:
	cd ${WRKSRC} && GOOS=openbsd ${MAKE_ENV} ${MAKE_PROGRAM} victoria-metrics-pure
	cd ${WRKSRC} && GOOS=openbsd ${MAKE_ENV} ${MAKE_PROGRAM} vmbackup
	cd ${WRKSRC} && GOOS=openbsd ${MAKE_ENV} ${MAKE_PROGRAM} vmrestore
	cd ${WRKSRC} && GOOS=openbsd ${MAKE_ENV} ${MAKE_PROGRAM} vmagent-pure

do-install:
	${INSTALL_PROGRAM} ./pkg/vmlogger.pl ${PREFIX}/bin/vmetricslogger.pl
	${INSTALL_PROGRAM} ${WRKSRC}/bin/victoria-metrics-pure ${PREFIX}/bin/vmetrics
	${INSTALL_PROGRAM} ${WRKSRC}/bin/vmagent-pure ${PREFIX}/bin/vmagent
	${INSTALL_PROGRAM} ${WRKSRC}/bin/vmbackup ${PREFIX}/bin/vmetricsbackup
	${INSTALL_PROGRAM} ${WRKSRC}/bin/vmrestore ${PREFIX}/bin/vmetricsrestore
	${INSTALL_DATA_DIR} ${PREFIX}/share/doc/vmetrics/
	${INSTALL_DATA} ${WRKSRC}/README.md ${PREFIX}/share/doc/vmetrics/
	${INSTALL_DATA} ${WRKSRC}/LICENSE ${PREFIX}/share/doc/vmetrics/
	${INSTALL_DATA} ${WRKSRC}/docs/* ${PREFIX}/share/doc/vmetrics/

.include <bsd.port.mk>
