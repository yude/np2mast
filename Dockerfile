FROM golang:latest AS builder

ADD ./ /src
WORKDIR /src
RUN go build .

FROM gcr.io/distroless/base-debian11:latest AS runner
COPY --from=builder /src/np2mast /bin/np2mast

WORKDIR /bin
CMD ["/bin/np2mast"]