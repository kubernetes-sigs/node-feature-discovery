# Create production image for funning node feature discovery
FROM debian:stretch-slim

ARG TMPDIR

COPY $TMPDIR/bin /usr/local/bin
COPY $TMPDIR/lib /usr/local/lib
RUN ldconfig
COPY $TMPDIR/node-feature-discovery /usr/bin/node-feature-discovery

ENTRYPOINT ["/usr/bin/node-feature-discovery"]
