### Build and run

The instruction below assume you don't have $GOPATH set. If you do replace ~/go with $GOPATH.

Clone
```
mkdir -p ~/go/src/github.com/pinpt
cd ~/go/src/github.com/pinpt
git clone git@github.com:pinpt/agent.git
```

#### Build natively

Build
```
cd ./agent
dep ensure -v
go install github.com/pinpt/agent
```

Update
```
git pull
dep ensure -v
go install github.com/pinpt/agent
```

Run
```
~/go/bin/agent enroll <CODE> --channel=dev
```

It will store the data into ~/.pinpoint/next folder.

If you want to re-enroll the agent, delete ~/.pinpoint/next.

#### Build and run via docker

```
docker build -t pinpoint-agent --build-arg VERSION=dev -f ./docker/release/Dockerfile .
docker run --rm pinpoint-agent enroll <CODE>
```