# Build DECtalk from the community source (dectalk/dectalk), pinned to a known commit
FROM debian:bookworm-slim AS dectalk
ARG DECTALK_REF=986ea12ec8dac85881394d7bd7daf1f1d5098b9a
# Ends with a self-test that runs say exactly like the server does, including non-ASCII text.
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential autoconf automake libtool pkg-config git ca-certificates \
      libasound2-dev libpulse-dev \
 && git init -q /src && cd /src \
 && git fetch -q --depth 1 https://github.com/dectalk/dectalk "$DECTALK_REF" && git checkout -q FETCH_HEAD \
 && cd src && ./autogen.sh && ./configure && make -j"$(nproc)" \
 && cp -r /src/dist /dectalk && rm -rf /dectalk/tools /dectalk/bitmaps \
 && cd /dectalk && printf 'Build test, 72\302\260F, caf\303\251.' \
      | LANG=C.UTF-8 ./say -c -fo stdout:raw - > /tmp/t.raw && test -s /tmp/t.raw

# Wyoming server: one static Go binary, stdlib only
FROM golang:1.23-bookworm AS server
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /dectalk-wyoming .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends libasound2 libpulse0 \
 && rm -rf /var/lib/apt/lists/*
# say decodes input with the locale's charset; without this, non-ASCII (°, é, ’) is dropped
ENV LANG=C.UTF-8
COPY --from=dectalk /dectalk /dectalk
COPY --from=server /dectalk-wyoming /usr/local/bin/dectalk-wyoming
CMD ["dectalk-wyoming"]
