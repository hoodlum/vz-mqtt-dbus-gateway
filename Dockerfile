ARG BINARY_NAME=vz-mqtt-dbus-gateway
FROM scratch
ARG BINARY_NAME
COPY ${BINARY_NAME} /gateway
ENTRYPOINT ["/gateway"]
