build-win:
	env GOOS=windows go build -o ./dist/win/agent-next.exe github.com/pinpt/agent.next
	env GOOS=windows go build -o ./dist/win/test.exe github.com/pinpt/agent.next/support/pluginhang 
	env GOOS=windows go build -o ./dist/win/aaihang.exe ./integration

test: export PP_AGENT_PLUGINHANG_BINS = ./dist/local

test:
	go build -o ./dist/local/agent-next github.com/pinpt/agent.next
	go build -o ./dist/local/test github.com/pinpt/agent.next/support/pluginhang 
	go build -o ./dist/local/aaihang ./integration
	./dist/local/test
	./dist/local/test check