FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3
ARG TARGETPLATFORM
WORKDIR /
COPY ${TARGETPLATFORM}/sreportal /sreportal
USER 65532:65532

ENTRYPOINT ["/sreportal"]
