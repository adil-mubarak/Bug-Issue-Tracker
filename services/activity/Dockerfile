FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/activity ./services/activity

FROM alpine:3.21
RUN adduser -D -H app
COPY --from=build /out/activity /usr/local/bin/activity
USER app
EXPOSE 8084
ENTRYPOINT ["activity"]
