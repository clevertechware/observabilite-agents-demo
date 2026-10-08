FROM rust:1.99.0-slim-trixie AS build
WORKDIR /src
COPY Cargo.toml Cargo.lock ./
COPY src ./src
RUN cargo build --release --locked

FROM gcr.io/distroless/cc-debian13:nonroot
COPY --from=build /src/target/release/pricing-service /pricing-service
EXPOSE 8080
ENTRYPOINT ["/pricing-service"]
