### forcekill

This is a test project for Windows, Mac, and Linux.

The purpose of this is to test what's the best way to kill a subprocess and all of its children. We first assumed that process.Kill() would do this, but that is not the case. After some research and trial and error, we decided that the best way was to call system commands. For Windows `taskkill /F /T /PID 123` and on Unix systems `pkill -P 123`, (where 123 is the process pid).

The `main.go` program runs `./level1/main.go` and this one runs `./level2/main.go`. In level2, there is a 50-second timer, which makes level2 and main wait. In main, we're killing level1 after one second, and level2 should be killed as well. In Mac or Linux, we can see in the logs when this happens, but on Windows, we'll have to watch the Task Manager since `taskkill` does not send a signal we can listen to.

The code should be self-explanatory, you can use the Makefile to run it. When in Windows, `make windows`, and when in Mac or Linux, `make mac`

