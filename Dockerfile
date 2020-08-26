FROM alpine:latest
ADD . /build
WORKDIR /build
RUN apk upgrade
RUN apk add perl autoconf automake libtool gettext gettext-dev make git gcc go upx
RUN git clone https://github.com/cretz/tor-static.git --recursive --depth=1
WORKDIR /build/tor-static
RUN go run build.go build-all
RUN strip -s ./tor/src/app/tor
RUN upx -9 ./tor/src/app/tor
