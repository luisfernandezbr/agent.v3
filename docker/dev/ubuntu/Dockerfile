FROM ubuntu:bionic

RUN apt-get update && \
    apt install -y wget git && \
    wget https://dl.google.com/go/go1.13.linux-amd64.tar.gz && \
    tar -C /usr/local -xzf go1.13.linux-amd64.tar.gz && \
    mkdir ~/go

ENV PATH="/usr/local/go/bin:${PATH}"

ENV GOPATH="/go"
    
ENTRYPOINT [ "/bin/bash" ]
