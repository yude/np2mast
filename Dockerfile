FROM golang:alpine AS builder

ADD ./ /src
WORKDIR /src
RUN go build .

FROM alpine:latest AS runner
COPY --from=builder /src/np2mast /bin/np2mast

CMD ["/bin/np2mast"]