# Build DECtalk from the community source (dectalk/dectalk)
FROM debian:bookworm-slim AS dectalk
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential autoconf automake libtool git ca-certificates \
      libasound2-dev libpulse-dev \
 && git clone --depth 1 https://github.com/dectalk/dectalk /src \
 && cd /src/src && ./autogen.sh && ./configure && make -j"$(nproc)" \
 && cp -r "$(dirname "$(find /src/dist -name say -type f | head -1)")" /dectalk \
 && cd /dectalk && ./say -fo /tmp/t.wav -a "build test" && test -s /tmp/t.wav

# Wyoming server: one static Go binary, stdlib only
FROM golang:1.23-bookworm AS server
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dectalk-wyoming .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libasound2 libpulse0 \
 && rm -rf /var/lib/apt/lists/*
COPY --from=dectalk /dectalk /dectalk
COPY --from=server /dectalk-wyoming /usr/local/bin/dectalk-wyoming
CMD ["dectalk-wyoming"]
