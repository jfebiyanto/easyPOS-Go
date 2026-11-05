FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/easypos ./cmd/easypos

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/easypos /easypos
USER 10001:10001
ENV TZ=Asia/Jakarta
EXPOSE 8080
ENTRYPOINT ["/easypos"]
CMD ["serve"]
