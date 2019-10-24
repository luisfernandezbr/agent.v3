### Build and run

The instruction below assume you don't have $GOPATH set. If you do replace ~/go with $GOPATH.

Clone
```
mkdir -p ~/go/src/github.com/pinpt
cd ~/go/src/github.com/pinpt
git clone git@github.com:pinpt/agent.next.git
```

Build
```
cd ./agent.next
dep ensure -v
go install github.com/pinpt/agent.next
```

Update
```
git pull
dep ensure -v
go install github.com/pinpt/agent.next
```

Run
```
~/go/bin/agent.next enroll <CODE> --channel=dev
~/go/bin/agent.next service-run
```

It will store the data into ~/.pinpoint/next folder.

If you want to re-enroll the agent, delete ~/.pinpoint/next.