FROM golang:1.27-alpine AS build
WORKDIR /src
ENV GOWORK=off GOPROXY=https://proxy.golang.org,direct
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -trimpath -o /quiet-service .

FROM alpine:3.23
RUN adduser -D -u 10001 quiet
USER quiet
COPY --from=build /quiet-service /quiet-service
ENTRYPOINT ["/quiet-service"]
