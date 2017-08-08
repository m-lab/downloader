###### EXAMPLE ######
# Chose golang container
FROM golang

# Copy files to correct gopath
ADD . /go/src/github.com/m-lab/downloader

# Build (this is where dependency management goes)
RUN go get github.com/m-lab/downloader/...
RUN go install github.com/m-lab/downloader

# Call the compiled program
#ENTRYPOINT /go/bin/downloader
#### END EXAMPLE ####

# Builds, gets dependencies, and runs
#from golang:onbuild
CMD /go/bin/downloader -bucket=${DOWNLOADER_BUCKET}
# Expose endpoint for prometheus metrics
EXPOSE 9090