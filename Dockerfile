FROM debian:bullseye

ENV BUILD_DIR=/usr/src/pdns-api
ENV DEBIAN_FRONTEND=noninteractive

# Add deb-src to sources.list
RUN find /etc/apt/sources.list* -type f -exec sed -i 'p; s/^deb /deb-src /' '{}' +

# Install developer tools
RUN apt-get update \
 && apt-get install --no-install-recommends -yV \
    apt-utils \
    build-essential \
    devscripts \
    equivs \
    vim

WORKDIR ${BUILD_DIR}
ADD . ${BUILD_DIR}

RUN mk-build-deps -irt 'apt-get --no-install-recommends -yV' /usr/src/pdns-api/debian/control
RUN dpkg-buildpackage -b -us -uc
