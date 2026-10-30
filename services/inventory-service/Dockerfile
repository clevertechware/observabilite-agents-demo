FROM golang:1.27.3-trixie AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/inventory-service .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/inventory-service /inventory-service
ENV GIN_MODE=release
EXPOSE 8080
ENTRYPOINT ["/inventory-service"]
