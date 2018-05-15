# fastSharerGo [![Build Status](https://travis-ci.com/monz/fastSharerGo.svg?branch=master)](https://travis-ci.com/monz/fastSharerGo)
 GoLang implementation of fastSharer, so it can be used on NAS systems 
 
## Install
```
go get github.com/monz/fastSharerGo/fastSharer
```

## Usage
To share all files in `share` directory and download files from other peers into `download` directory use the following command.
```
$ fastSharer -downloadDir download -shareDir share
```
> Currently only files which were in the directory on fastSharer startup will be considered for sharing. Newly added files will be ignored.

## Cross Compile
For ARM based NAS systems compile the programm as follows.
```
env GOOS=linux GOARCH=arm go build fastSharer.go
```
Or choose `GOOS` and `GOARCH` combination of your needs from here: https://golang.org/doc/install/source#environment
