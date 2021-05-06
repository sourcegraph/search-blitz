FROM golang AS BUILDER
WORKDIR /build
COPY go.sum go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o searchblitz .

FROM sourcegraph/alpine:3.12@sha256:ce099fbcd3cf70b338fc4cb2a4e1fa9ae847de21afdb0a849a393b87d94fb174
RUN mkdir data

COPY --from=builder /build/searchblitz /usr/local/bin
COPY data data

ARG COMMIT_SHA="unknown"

LABEL org.opencontainers.image.revision=${COMMIT_SHA}
LABEL org.opencontainers.image.source=https://github.com/sourcegraph/search-blitz/

ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/searchblitz"]
