FROM gcr.io/distroless/static-debian13:nonroot
LABEL org.opencontainers.image.source=https://github.com/siemenvdn/zigbeat
ARG TARGETPLATFORM
COPY $TARGETPLATFORM/zigbeat /usr/bin/zigbeat
WORKDIR /config
ENTRYPOINT ["/usr/bin/zigbeat"]
CMD ["serve"]
