FROM lu4p/xgo-custom

RUN apt-get update && apt-get upgrade -y && apt-get install -y tor build-essential libtool autopoint wget unzip
RUN mkdir -p /go/src/github.com/cretz/
RUN cd /go/src/github.com/cretz/ && git clone https://github.com/cretz/tor-static.git --recursive
RUN cd /go/src/github.com/cretz/tor-static && go run build.go build-all
RUN cd /go/src/github.com/cretz/tor-static && go run build.go package-libs
RUN cd /go/src/github.com/cretz/tor-static && rm libs.zip && mv libs.tar.gz /libs_linux.tar.gz
#get windows libs
RUN cd /go/src/github.com/cretz/tor-static  && wget https://github.com/lu4p/tor-static/releases/download/2/tor-static-windows-amd64.zip
