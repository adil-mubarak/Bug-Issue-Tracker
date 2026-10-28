FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/project ./services/project

FROM alpine:3.21
RUN adduser -D -H app
COPY --from=build /out/project /usr/local/bin/project
USER app
EXPOSE 8082
ENTRYPOINT ["project"]
