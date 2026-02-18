FROM gcr.io/distroless/static:nonroot
ARG TARGETPLATFORM
WORKDIR /
COPY ${TARGETPLATFORM}/sreportal /sreportal
USER 65532:65532

ENTRYPOINT ["/sreportal"]
