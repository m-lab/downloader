FROM golang:1.13 as builder
# Set up the build environment
ENV CGO_ENABLED 0
# Copy files to correct gopath
ADD . /go/src/github.com/m-lab/downloader
WORKDIR /go/src/github.com/m-lab/downloader
# Get dependencies
RUN go get -v ./...
# Build
RUN go install \
      -v \
      -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h)" \
      ./...


# Set up the image we will eventually use
FROM alpine
# By default, alpine has no root certs.  We need them for authenticating to GCS.
RUN apk add --no-cache ca-certificates && update-ca-certificates

# Bring the binary over from the builder image.
COPY --from=builder /go/bin/downloader /bin/downloader

# TODO: Convert this to use ENTRYPOINT and update the argument settings in the
# k8s config.
CMD /bin/downloader -bucket=${DOWNLOADER_BUCKET} -project=${PROJECT_NAME} --prometheusx.listen-address=:9090
# Expose endpoint for prometheus metrics
EXPOSE 9090
