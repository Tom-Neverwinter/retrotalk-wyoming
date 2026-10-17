# The Wyoming server and its speech engine: one static Go binary, standard library only.
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY *.go ./
COPY tts ./tts
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /tts-server .

FROM scratch
COPY --from=build /tts-server /tts-server
# CMUdict is embedded in the binary; its BSD licence must travel with it.
COPY tts/CMUDICT-LICENSE /licenses/CMUDICT-LICENSE
CMD ["/tts-server"]
