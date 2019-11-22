FROM golang:1.12 as deps

RUN mkdir build 

COPY go.mod build/go.mod
COPY go.sum build/go.sum

RUN echo "$GOPATH" && \
    cd build && \
    go get

FROM golang:1.12 as builder

# NAME and VERSION are the name of the software in releases.hashicorp.com
# and the version to download. Example: NAME=consul VERSION=1.2.3.
ARG NAME
ARG VERSION

# Set ARGs as ENV so that they can be used in ENTRYPOINT/CMD
ENV NAME=$NAME
ENV VERSION=$VERSION

COPY --from=deps /go /go
COPY . /src
RUN cd /src && GOOS=linux GOARCH=amd64 CGO_ENABLED=0  go build -o ${NAME}

FROM alpine:3.8

ARG NAME
ARG VERSION

ENV NAME=$NAME
ENV VERSION=$VERSION

# Create a non-root user to run the software.
RUN echo ${NAME}
RUN addgroup ${NAME} && \
    adduser -S -G ${NAME} ${NAME}

COPY --from=builder /src/${NAME} /bin/

USER ${NAME}

CMD /bin/${NAME}
