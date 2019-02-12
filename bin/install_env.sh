#!/usr/bin/env bash

echo "Apt update upgrade"
apt update
apt -y upgrade

echo "Install wget git"
apt install -y wget git

if [ ! -d "/usr/local/go" ]; then
    echo "Install golang..."
    wget https://dl.google.com/go/go1.11.5.linux-amd64.tar.gz
    tar -xvf go1.11.5.linux-amd64.tar.gz
    mv go /usr/local
    echo "Install golang... DONE"
else
    echo "Golang is installed"
fi

mkdir ~/go/bin -p
if !(grep -q "GOROOT" ~/.bashrc); then
    echo "Setup env GOROOT GOPATH..."
    echo 'export GOROOT=/usr/local/go' >> ~/.bashrc
    echo 'export GOPATH=$HOME/go' >> ~/.bashrc
    echo 'export PATH=$GOPATH/bin:$GOROOT/bin:$PATH' >> ~/.bashrc
    echo "Setup env GOROOT GOPATH... DONE"
fi
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH

if [ ! -f ~/go/bin/dep ]; then
    echo "Install dep..."
    go get -u github.com/golang/dep/cmd/dep
    echo "Install dep... DONE"
else
    echo "Dep is installed"
fi

mkdir ~/go/src/github.com/ninjadotorg -p
cd ~/go/src/github.com/ninjadotorg
if [ ! -d constant ]; then
    echo "Clone constant..."
    git clone https://github.com/ninjadotorg/constant -b master
    echo "Clone constant... DONE"
else
    echo "Constant directory is existed"
    git pull
fi

echo "Install constant packages..."
cd constant
dep ensure -v
echo "Install constant packages... DONE"

cd ~/go/src/github.com/ninjadotorg/constant/bin
