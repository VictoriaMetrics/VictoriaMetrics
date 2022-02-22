FROM snapcore/snapcraft:stable
ARG GO_VERSION
RUN apt-get update && apt-get install  -y git make wget  binutils build-essential bzip2 cpp cpp-5 dpkg-dev fakeroot g++ g++-5 gcc gcc-5

RUN cd /usr/local &&\
    wget https://dl.google.com/go/go$GO_VERSION.linux-amd64.tar.gz &&\
    tar -zxvf go$GO_VERSION.linux-amd64.tar.gz && rm go$GO_VERSION.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}" 

